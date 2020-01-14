package units

import (
	"context"
	"fmt"
	"sort"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/perf"
	"github.com/evergreen-ci/cedar/util"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/sometimes"
	"github.com/pkg/errors"
)

type RecalculateChangePointsJob struct {
	*job.Base           `bson:"metadata" json:"metadata" yaml:"metadata"`
	env                 cedar.Environment
	conf                *model.CedarConfig
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
		env: cedar.GetEnvironment(),
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

func (j *RecalculateChangePointsJob) makeMessage(msg string, id model.TimeSeriesId) message.Fields {
	return message.Fields{
		"job_id":  j.ID(),
		"message": msg,
		"project": id.Project,
		"variant": id.Variant,
		"task":    id.Task,
		"test":    id.Test,
	}
}

func (j *RecalculateChangePointsJob) Run(ctx context.Context) {
	defer j.MarkComplete()
	if j.conf == nil {
		j.conf = model.NewCedarConfig(j.env)
	}
	if j.conf.Flags.DisableSignalProcessing {
		grip.InfoWhen(sometimes.Percent(10), j.makeMessage("signal processing is disabled, skipping processing", j.TimeSeriesId))
		return
	}
	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}
	if j.ChangePointDetector == nil {
		err := j.conf.Find()
		if err != nil {
			j.AddError(errors.Wrap(err, "Unable to get cedar configuration"))
			return
		}
		j.ChangePointDetector = perf.NewMicroServiceChangeDetector(j.conf.ChangeDetector.URI, j.conf.ChangeDetector.User, j.conf.ChangeDetector.Token)
	}
	timeSeries, err := model.GetTimeSeries(ctx, j.env, j.TimeSeriesId)
	if err != nil {
		j.AddError(errors.Wrapf(err, "Unable to aggregate time series %s", j.TimeSeriesId))
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
		j.AddError(errors.Wrapf(err, "Unable to detect change points in time series %s", j.TimeSeriesId))
		return
	}

	j.AddError(model.ReplaceChangePoints(ctx, j.env, timeSeries, changePoints))
	j.AddError(model.MarkMetricAsAnalyzed(ctx, j.env, timeSeries.TimeSeriesId))
}
