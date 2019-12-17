package units

import (
	"context"
	"fmt"
	"sort"

	"github.com/mongodb/grip"

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
	*job.Base    `bson:"metadata" json:"metadata" yaml:"metadata"`
	env          cedar.Environment
	TimeSeriesId model.TimeSeriesId `bson:"time_series_id" json:"time_series_id" yaml:"time_series_id"`
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

func NewRecalculateChangePointsJob(timeSeriesId model.TimeSeriesId) amboy.Job {
	j := makeChangePointsJob()
	// Every ten minutes at most
	timestamp := util.RoundPartOfHour(10)
	j.SetID(fmt.Sprintf("%s.%s.%s.%s.%s.%s.%s", j.JobType.Name, timeSeriesId.Id.Project, timeSeriesId.Id.Variant, timeSeriesId.Id.Task, timeSeriesId.Id.Test, timeSeriesId.Id.Measurement, timestamp))
	j.TimeSeriesId = timeSeriesId
	return j
}

func makeMessage(msg string, id model.TimeSeriesId) message.Fields {
	return message.Fields{
		"message": msg,
		"project": id.Id.Project,
		"variant": id.Id.Variant,
		"task":    id.Id.Task,
		"test":    id.Id.Test,
	}
}

func (j *recalculateChangePointsJob) Run(ctx context.Context) {
	defer j.MarkComplete()
	conf := model.NewCedarConfig(j.env)
	err := conf.Find()
	if err != nil {
		message.WrapError(err, message.Fields{
			"message": "Unable to get cedar configuration",
		})
		return
	}
	if conf.Flags.DisableSignalProcessing == true {
		grip.Info(makeMessage("signal processing is disabled, skipping processing", j.TimeSeriesId))
		return
	}
	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}
	db := j.env.GetDB()
	detector := perf.NewMicroServiceChangeDetector(conf.ChangeDetector.URI, conf.ChangeDetector.User, conf.ChangeDetector.Token)
	cur, err := model.GetTimeSeries(ctx, db, j.TimeSeriesId)
	defer cur.Close(ctx)
	if err != nil {
		message.WrapError(err, makeMessage("Unable to aggregate time series", j.TimeSeriesId))
		return
	}
	for cur.Next(ctx) {
		var result model.TimeSeries
		err = cur.Decode(result)
		if err != nil {
			message.WrapError(err, makeMessage("Unable to decode aggregated time series", j.TimeSeriesId))
			break
		}

		sort.Slice(result.Data, func(i, j int) bool {
			return result.Data[i].Order < result.Data[j].Order
		})

		var series []float64

		for _, item := range result.Data {
			series = append(series, item.Value)
		}

		changePoints, err := detector.DetectChanges(ctx, series)
		if err != nil {
			message.WrapError(err, makeMessage("Unable to detect change points in time series", j.TimeSeriesId))
			continue
		}

		err = model.ClearChangePoints(ctx, db, j.TimeSeriesId)
		if err != nil {
			message.WrapError(err, makeMessage("Unable to clear change points for measurement", j.TimeSeriesId))
			continue
		}

		for _, cp := range changePoints {
			perfResultId := result.Data[cp.Index].PerfResultID
			err = model.CreateChangePoint(ctx, db, perfResultId, j.TimeSeriesId.Id.Measurement, cp.Info)
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
