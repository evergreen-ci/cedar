package units

import (
	"context"
	"fmt"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/perf"
	"github.com/mongodb/grip"
	"sort"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/util"
	"gopkg.in/mgo.v2/bson"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
)

type recalculateChangePointsJob struct {
	*job.Base      `bson:"metadata" json:"metadata" yaml:"metadata"`
	env            cedar.Environment
	metricGrouping MetricGrouping
}

func init() {
	registry.AddJobType("hello-world", helloWorldJobFactory)
}

func recalculateChangePointsJobFactory() *recalculateChangePointsJob {
	j := &recalculateChangePointsJob{
		Base: &job.Base{
			JobType: amboy.JobType{
				Name:    "recalculate-change-points",
				Version: 1,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func NewRecalculateChangePointsJob(metricGrouping MetricGrouping) amboy.Job {
	j := recalculateChangePointsJobFactory()
	// Every ten minutes at most
	timestamp := util.RoundPartOfHour(10)
	j.SetID(fmt.Sprintf("%s.%s.%s.%s.%s.%s", j.JobType.Name, metricGrouping.Id.Project, metricGrouping.Id.Variant, metricGrouping.Id.Task, metricGrouping.Id.Test, timestamp))
	j.metricGrouping = metricGrouping
	return j
}

type MetricGrouping struct {
	Id struct {
		Project string `bson:"project"`
		Variant string `bson:"variant"`
		Task    string `bson:"task"`
		Test    string `bson:"test"`
	} `bson:"_id"`
}

func makeSeriesFilter(grouping MetricGrouping) bson.M {
	return bson.M{
		"info.project":   grouping.Id.Project,
		"info.variant":   grouping.Id.Variant,
		"info.task_name": grouping.Id.Task,
		"info.test_name": grouping.Id.Test,
	}

}

func makeTimeSeriesPipeline(grouping MetricGrouping) []bson.M {
	var pipeline []bson.M

	pipeline = append(pipeline, bson.M{
		"match": makeSeriesFilter(grouping),
	})

	pipeline = append(pipeline, bson.M{
		"$match": bson.M{
			"info.order": bson.M{
				"$exists": true,
			},
			"info.mainline": true,
		},
	})

	pipeline = append(pipeline, bson.M{
		"$unwind": "$rollups.stats",
	})

	pipeline = append(pipeline, bson.M{
		"$project": bson.M{
			"measurement": "$rollups.stats.name",
			"value":       "$rollups.stats.val",
			"order":       "$info.order",
		},
	})

	pipeline = append(pipeline, bson.M{
		"$group": bson.M{
			"_id": "$measurement",
			"time_series": bson.M{
				"$push": bson.M{
					"id":    "$_id",
					"value": "$value",
					"order": "$order",
				},
			},
		},
	})

	return pipeline
}

type errorMsg = map[string]interface{}

type timeSeriesElement struct {
	PerfResultID string `bson:"id"`
	Value        float64
	Order        int
}

type measurementTimeSeries struct {
	Measurement string              `bson:"_id"`
	TimeSeries  []timeSeriesElement `bson:"times_series"`
}

func (j *recalculateChangePointsJob) Run(ctx context.Context) {
	defer j.MarkComplete()
	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}
	conf := model.NewCedarConfig(j.env)
	err := conf.Find()
	if err != nil {
		grip.Error(errorMsg{
			"message": "Unable to get cedar configuration",
			"error":   err,
		})
	}
	detector := perf.NewMicroServiceChangeDetector(conf.ChangeDetector.URI, conf.ChangeDetector.User, conf.ChangeDetector.Token)
	collection := j.env.GetDB().Collection("perf_results")
	timeSeriesAggregator := makeTimeSeriesPipeline(j.metricGrouping)
	cur, err := collection.Aggregate(ctx, timeSeriesAggregator)
	defer cur.Close(ctx)
	if err != nil {
		grip.Error(errorMsg{
			"message": "Unable to aggregate time series",
			"project": j.metricGrouping.Id.Project,
			"variant": j.metricGrouping.Id.Variant,
			"task":    j.metricGrouping.Id.Task,
			"test":    j.metricGrouping.Id.Test,
			"error":   err,
		})
		return
	}
	for cur.Next(ctx) {
		var result measurementTimeSeries

		err = cur.Decode(result)
		if err != nil {
			grip.Error(errorMsg{
				"message": "Unable to decode aggregated time series",
				"project": j.metricGrouping.Id.Project,
				"variant": j.metricGrouping.Id.Variant,
				"task":    j.metricGrouping.Id.Task,
				"test":    j.metricGrouping.Id.Test,
				"error":   err,
			})
			continue
		}

		sort.Slice(result.TimeSeries, func(i, j int) bool {
			return result.TimeSeries[i].Order < result.TimeSeries[j].Order
		})

		var series []float64

		for _, item := range result.TimeSeries {
			series = append(series, item.Value)
		}

		changePoints, err := detector.DetectChanges(ctx, series)
		if err != nil {
			grip.Error(errorMsg{
				"message":     "Unable to detect change points in time series",
				"project":     j.metricGrouping.Id.Project,
				"variant":     j.metricGrouping.Id.Variant,
				"task":        j.metricGrouping.Id.Task,
				"test":        j.metricGrouping.Id.Test,
				"measurement": result.Measurement,
				"error":       err,
			})
			continue
		}

		seriesFilter := makeSeriesFilter(j.metricGrouping)
		pullUpdate := bson.M{
			"$pull": bson.M{
				"change_points": bson.M{
					"measurement": result.Measurement,
				},
			},
		}
		_, err = collection.UpdateMany(ctx, seriesFilter, pullUpdate)
		if err != nil {
			grip.Error(errorMsg{
				"message":     "Unable to clear change points for measurement",
				"project":     j.metricGrouping.Id.Project,
				"variant":     j.metricGrouping.Id.Variant,
				"task":        j.metricGrouping.Id.Task,
				"test":        j.metricGrouping.Id.Test,
				"measurement": result.Measurement,
				"error":       err,
			})
		}

		for _, cp := range changePoints {
			perfResultId := result.TimeSeries[cp.Index].PerfResultID
			filter := bson.M{"_id": perfResultId}
			newChangePoint := bson.M{
				"measurement":   result.Measurement,
				"algorithm":     cp.Info,
				"calculated_at": time.Now(),
			}
			update := bson.M{
				"$push": bson.M{
					"change_points": newChangePoint,
				},
			}
			_, err := collection.UpdateOne(ctx, filter, update)
			if err != nil {
				grip.Error(errorMsg{
					"message":        "Failed to update performance result with change point",
					"perf_result_id": perfResultId,
					"change_point":   newChangePoint,
				})
			}
		}
	}
}
