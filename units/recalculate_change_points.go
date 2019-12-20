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

type RecalculateChangePointsJob struct {
	*job.Base           `bson:"metadata" json:"metadata" yaml:"metadata"`
	env                 cedar.Environment
	TimeSeriesId        model.TimeSeriesId `bson:"time_series_id" json:"time_series_id" yaml:"time_series_id"`
	ChangePointDetector perf.ChangeDetector
}

func init() {
	registry.AddJobType("recalculate-change-points", func() amboy.Job { return makeChangePointsJob() })
}

func makeChangePointsJob() *RecalculateChangePointsJob {
	j := &RecalculateChangePointsJob{
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
	j.SetID(fmt.Sprintf("%s.%s.%s.%s.%s.%s.%s", j.JobType.Name, timeSeriesId.Project, timeSeriesId.Variant, timeSeriesId.Task, timeSeriesId.Test, timeSeriesId.Measurement, timestamp))
	j.TimeSeriesId = timeSeriesId
	return j
}

func makeMessage(msg string, id model.TimeSeriesId) message.Fields {
	return message.Fields{
		"message": msg,
		"project": id.Project,
		"variant": id.Variant,
		"task":    id.Task,
		"test":    id.Test,
	}
}

func (j *RecalculateChangePointsJob) Run(ctx context.Context) {
	defer j.MarkComplete()
	conf := model.NewCedarConfig(j.env)
	err := conf.Find()
	if err != nil {
		grip.Error(message.WrapError(err, message.Fields{
			"message": "Unable to get cedar configuration",
		}))
		return
	}
	if conf.Flags.DisableSignalProcessing == true {
		grip.Info(makeMessage("signal processing is disabled, skipping processing", j.TimeSeriesId))
		return
	}
	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}
	if j.ChangePointDetector == nil {
		j.ChangePointDetector = perf.NewMicroServiceChangeDetector(conf.ChangeDetector.URI, conf.ChangeDetector.User, conf.ChangeDetector.Token)
	}
	timeSeries, err := model.GetTimeSeries(ctx, j.env, j.TimeSeriesId)
	if err != nil {
		grip.Error(message.WrapError(err, makeMessage("Unable to aggregate time series", j.TimeSeriesId)))
		return
	}
	sort.Slice(timeSeries.Data, func(i, j int) bool {
		return timeSeries.Data[i].Order < timeSeries.Data[j].Order
	})

	var series []float64
	for _, item := range timeSeries.Data {
		series = append(series, item.Value)
	}

	changePoints, err := j.ChangePointDetector.DetectChanges(ctx, series)
	if err != nil {
		grip.Error(message.WrapError(err, makeMessage("Unable to detect change points in time series", j.TimeSeriesId)))
		return
	}

	err = model.ClearChangePoints(ctx, j.env, j.TimeSeriesId)
	if err != nil {
		grip.Error(message.WrapError(err, makeMessage("Unable to clear change points for measurement", j.TimeSeriesId)))
		return
	}
	for _, cp := range changePoints {
		perfResultId := timeSeries.Data[cp.Index].PerfResultID
		err = model.CreateChangePoint(ctx, j.env, perfResultId, j.TimeSeriesId.Measurement, cp.Info)
		if err != nil {
			grip.Error(message.WrapError(err, message.Fields{
				"message":        "Failed to update performance result with change point",
				"perf_result_id": perfResultId,
				"change_point":   cp,
			}))
		}
	}
}
