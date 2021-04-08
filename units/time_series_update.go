package units

import (
	"context"
	"fmt"
	"sort"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/perf"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/sometimes"
	"github.com/pkg/errors"
)

type timeSeriesUpdateJob struct {
	*job.Base                  `bson:"metadata" json:"metadata" yaml:"metadata"`
	env                        cedar.Environment
	conf                       *model.CedarConfig
	PerformanceResultId        model.PerformanceResultSeriesID `bson:"time_series_id" json:"time_series_id" yaml:"time_series_id"`
	performanceAnalysisService perf.PerformanceAnalysisService
}

func init() {
	registry.AddJobType("time-series-update", func() amboy.Job { return makeTimeSeriesJob() })
}

func makeTimeSeriesJob() *timeSeriesUpdateJob {
	j := &timeSeriesUpdateJob{
		Base: &job.Base{
			JobType: amboy.JobType{
				Name:    "time-series-update",
				Version: 1,
			},
		},
		env: cedar.GetEnvironment(),
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

// NewUpdateTimeSeriesJob creates a new amboy job to update a time series.
func NewUpdateTimeSeriesJob(timeSeriesId model.PerformanceResultSeriesID) amboy.Job {
	j := makeTimeSeriesJob()
	// Every ten minutes at most
	timestamp := utility.RoundPartOfHour(10)
	j.SetID(fmt.Sprintf("%s.%s.%s.%s.%s.%s", j.JobType.Name, timeSeriesId.Project, timeSeriesId.Variant, timeSeriesId.Task, timeSeriesId.Test, timestamp))
	j.PerformanceResultId = timeSeriesId
	return j
}

func (j *timeSeriesUpdateJob) makeMessage(msg string, id model.PerformanceResultSeriesID) message.Fields {
	return message.Fields{
		"job_id":  j.ID(),
		"message": msg,
		"project": id.Project,
		"variant": id.Variant,
		"task":    id.Task,
		"test":    id.Test,
	}
}

func (j *timeSeriesUpdateJob) Run(ctx context.Context) {
	defer j.MarkComplete()
	if j.conf == nil {
		j.conf = model.NewCedarConfig(j.env)
	}
	if j.conf.Flags.DisableSignalProcessing {
		grip.InfoWhen(sometimes.Percent(10), j.makeMessage("signal processing is disabled, skipping processing", j.PerformanceResultId))
		return
	}
	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}
	if j.performanceAnalysisService == nil {
		err := j.conf.Find()
		if err != nil {
			j.AddError(errors.Wrap(err, "Unable to get cedar configuration"))
			return
		}
		j.performanceAnalysisService = perf.NewPerformanceAnalysisService(j.conf.ChangeDetector.URI, j.conf.ChangeDetector.User, j.conf.ChangeDetector.Token)
	}
	performanceData, err := model.GetPerformanceData(ctx, j.env, j.PerformanceResultId)
	if err != nil {
		j.AddError(errors.Wrapf(err, "Unable to aggregate time perfData %s", j.PerformanceResultId.String()))
		return
	}
	if performanceData == nil {
		j.AddError(model.MarkPerformanceResultsAsAnalyzed(ctx, j.env, j.PerformanceResultId))
		return
	}
	for _, perfData := range performanceData {
		sort.Slice(perfData.TimeSeries, func(i, j int) bool {
			return perfData.TimeSeries[i].Order < perfData.TimeSeries[j].Order
		})
		series := perf.TimeSeriesModel{
			Project:     perfData.PerformanceResultId.Project,
			Variant:     perfData.PerformanceResultId.Variant,
			Task:        perfData.PerformanceResultId.Task,
			Test:        perfData.PerformanceResultId.Test,
			Measurement: perfData.PerformanceResultId.Measurement,
		}
		series.Arguments = []perf.ArgumentsModel{}
		for k, v := range perfData.PerformanceResultId.Arguments {
			series.Arguments = append(series.Arguments, perf.ArgumentsModel{
				Name:  k,
				Value: v,
			})
		}
		for _, item := range perfData.TimeSeries {
			series.Data = append(series.Data, perf.TimeSeriesDataModel{
				PerformanceResultID: item.PerfResultID,
				Order:               item.Order,
				Value:               item.Value,
				Version:             item.Version,
			})
		}
		err := j.performanceAnalysisService.ReportUpdatedTimeSeries(ctx, series)
		if err != nil {
			j.AddError(errors.Wrapf(err, "Unable to update time series for perfData %s", j.PerformanceResultId.String()))
			return
		}
		j.AddError(model.MarkPerformanceResultsAsAnalyzed(ctx, j.env, perfData.PerformanceResultId))
	}
}
