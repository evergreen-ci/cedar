package units

import (
	"context"
	"fmt"
	"sort"

	"github.com/mongodb/grip/message"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/perf"
	"github.com/evergreen-ci/cedar/util"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
)

type recalculateChangePointsJob struct {
	*job.Base      `bson:"metadata" json:"metadata" yaml:"metadata"`
	env            cedar.Environment
	MetricGrouping model.MetricGrouping
}

func init() {
	registry.AddJobType("recalculate-change-points", func() amboy.Job { return makeChangePointsJob() })
}

func makeChangePointsJob() *recalculateChangePointsJob {
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

func NewRecalculateChangePointsJob(metricGrouping model.MetricGrouping) amboy.Job {
	j := makeChangePointsJob()
	// Every ten minutes at most
	timestamp := util.RoundPartOfHour(10)
	j.SetID(fmt.Sprintf("%s.%s.%s.%s.%s.%s", j.JobType.Name, metricGrouping.Id.Project, metricGrouping.Id.Variant, metricGrouping.Id.Task, metricGrouping.Id.Test, timestamp))
	j.MetricGrouping = metricGrouping
	return j
}

func (j *recalculateChangePointsJob) Run(ctx context.Context) {
	defer j.MarkComplete()
	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}
	db := j.env.GetDB()
	conf := model.NewCedarConfig(j.env)
	err := conf.Find()
	if err != nil {
		message.WrapError(err, message.Fields{
			"message": "Unable to get cedar configuration",
		})
		return
	}
	detector := perf.NewMicroServiceChangeDetector(conf.ChangeDetector.URI, conf.ChangeDetector.User, conf.ChangeDetector.Token)
	cur, err := model.GetTimeSeries(ctx, db, j.MetricGrouping)
	defer cur.Close(ctx)
	if err != nil {
		message.WrapError(err, message.Fields{
			"message": "Unable to aggregate time series",
			"project": j.MetricGrouping.Id.Project,
			"variant": j.MetricGrouping.Id.Variant,
			"task":    j.MetricGrouping.Id.Task,
			"test":    j.MetricGrouping.Id.Test,
		})
		return
	}
	for cur.Next(ctx) {
		var result model.MeasurementTimeSeries
		err = cur.Decode(result)
		if err != nil {
			message.WrapError(err, message.Fields{
				"message": "Unable to decode aggregated time series",
				"project": j.MetricGrouping.Id.Project,
				"variant": j.MetricGrouping.Id.Variant,
				"task":    j.MetricGrouping.Id.Task,
				"test":    j.MetricGrouping.Id.Test,
			})
			break
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
			message.WrapError(err, message.Fields{
				"message":     "Unable to detect change points in time series",
				"project":     j.MetricGrouping.Id.Project,
				"variant":     j.MetricGrouping.Id.Variant,
				"task":        j.MetricGrouping.Id.Task,
				"test":        j.MetricGrouping.Id.Test,
				"measurement": result.Measurement,
			})
			continue
		}

		err = model.ClearChangePoints(ctx, db, j.MetricGrouping, result.Measurement)
		if err != nil {
			message.WrapError(err, message.Fields{
				"message":     "Unable to clear change points for measurement",
				"project":     j.MetricGrouping.Id.Project,
				"variant":     j.MetricGrouping.Id.Variant,
				"task":        j.MetricGrouping.Id.Task,
				"test":        j.MetricGrouping.Id.Test,
				"measurement": result.Measurement,
			})
			continue
		}

		for _, cp := range changePoints {
			perfResultId := result.TimeSeries[cp.Index].PerfResultID
			err = model.CreateChangePoint(ctx, db, perfResultId, result.Measurement, cp.Info)
			if err != nil {
				message.WrapError(err, message.Fields{
					"message":        "Failed to update performance result with change point",
					"perf_result_id": perfResultId,
					"change_point":   cp,
				})
			}
		}
	}
}
