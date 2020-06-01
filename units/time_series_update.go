package units

import (
	"context"
	"fmt"
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
	//performanceData, err := model.GetPerformanceData(ctx, j.env, j.PerformanceResultId)
	//if err != nil {
	//	j.AddError(errors.Wrapf(err, "Unable to aggregate time perfData for project %s", j.PerformanceResultId.Project))
	//	return
	//}
	//if performanceData == nil {
	//	j.AddError(model.MarkPerformanceResultsAsAnalyzed(ctx, j.env, j.PerformanceResultId))
	//	return
	//}
	//for _, perfData := range performanceData.Data {
	//sort.Slice(perfData.TimeSeries, func(i, j int) bool {
	//	return perfData.TimeSeries[i].Order < perfData.TimeSeries[j].Order
	//})
	//series := perf.TimeSeriesModel{
	//	Project:     performanceData.PerformanceResultId.Project,
	//	Variant:     performanceData.PerformanceResultId.Variant,
	//	Task:        performanceData.PerformanceResultId.Task,
	//	Test:        performanceData.PerformanceResultId.Test,
	//	Measurement: perfData.Measurement,
	//}
	//for k, v := range performanceData.PerformanceResultId.Arguments {
	//	series.Arguments = append(series.Arguments, perf.ArgumentsModel{
	//		Name:  k,
	//		Value: v,
	//	})
	//}
	//for _, item := range perfData.TimeSeries {
	//	series.Data = append(series.Data, perf.TimeSeriesDataModel{
	//		PerformanceResultID: item.PerfResultID,
	//		Order:               item.Order,
	//		Value:               item.Value,
	//		Version:             item.Version,
	//	})
	//}
	var data []perf.TimeSeriesDataModel
	for i := 0; i < 50; i++ {
		data = append(data, perf.TimeSeriesDataModel{
			PerformanceResultID: "test1",
			Order:               0,
			Value:               10,
			Version:             "version1",
		})
	}
	for i := 0; i < 50; i++ {
		data = append(data, perf.TimeSeriesDataModel{
			PerformanceResultID: "test2",
			Order:               0,
			Value:               100,
			Version:             "version2",
		})
	}
	for i := 0; i < 50; i++ {
		data = append(data, perf.TimeSeriesDataModel{
			PerformanceResultID: "test3",
			Order:               0,
			Value:               500,
			Version:             "version3",
		})
	}
	for i := 0; i < 50; i++ {
		data = append(data, perf.TimeSeriesDataModel{
			PerformanceResultID: "test4",
			Order:               0,
			Value:               50,
			Version:             "version4",
		})
	}
	series := perf.TimeSeriesModel{
		Project:     "projecta",
		Variant:     "varianta",
		Task:        "task",
		Test:        "test",
		Measurement: "measuremnt",
		Arguments:   []perf.ArgumentsModel{},
		Data:        data,
	}

	err := j.performanceAnalysisService.ReportUpdatedTimeSeries(ctx, series)
	if err != nil {
		j.AddError(errors.Wrapf(err, "Unable to update time series for perfData for project for %s", j.PerformanceResultId.Project))
		return
	}
	//}
}
