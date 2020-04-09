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

type recalculateChangePointsJob struct {
	*job.Base           `bson:"metadata" json:"metadata" yaml:"metadata"`
	env                 cedar.Environment
	conf                *model.CedarConfig
	PerformanceResultId model.PerformanceResultSeriesID `bson:"time_series_id" json:"time_series_id" yaml:"time_series_id"`
	changePointDetector perf.ChangeDetector
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
		env: cedar.GetEnvironment(),
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func NewRecalculateChangePointsJob(timeSeriesId model.PerformanceResultSeriesID) amboy.Job {
	j := makeChangePointsJob()
	// Every ten minutes at most
	timestamp := util.RoundPartOfHour(10)
	j.SetID(fmt.Sprintf("%s.%s.%s.%s.%s.%s", j.JobType.Name, timeSeriesId.Project, timeSeriesId.Variant, timeSeriesId.Task, timeSeriesId.Test, timestamp))
	j.PerformanceResultId = timeSeriesId
	return j
}

func (j *recalculateChangePointsJob) makeMessage(msg string, id model.PerformanceResultSeriesID) message.Fields {
	return message.Fields{
		"job_id":  j.ID(),
		"message": msg,
		"project": id.Project,
		"variant": id.Variant,
		"task":    id.Task,
		"test":    id.Test,
	}
}

func (j *recalculateChangePointsJob) Run(ctx context.Context) {
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
	if j.changePointDetector == nil {
		err := j.conf.Find()
		if err != nil {
			j.AddError(errors.Wrap(err, "Unable to get cedar configuration"))
			return
		}
		j.changePointDetector = perf.NewMicroServiceChangeDetector(j.conf.ChangeDetector.URI, j.conf.ChangeDetector.User, j.conf.ChangeDetector.Token, perf.CreateDefaultAlgorithm())
	}
	performanceData, err := model.GetPerformanceData(ctx, j.env, j.PerformanceResultId)
	if err != nil {
		j.AddError(errors.Wrapf(err, "Unable to aggregate time series %s", j.PerformanceResultId))
		return
	}
	if performanceData == nil {
		j.AddError(model.MarkPerformanceResultsAsAnalyzed(ctx, j.env, j.PerformanceResultId))
		return
	}
	mappedChangePoints := map[string][]model.ChangePoint{}
	for _, series := range performanceData.Data {
		sort.Slice(series.TimeSeries, func(i, j int) bool {
			return series.TimeSeries[i].Order < series.TimeSeries[j].Order
		})
		floatSeries := make([]float64, len(series.TimeSeries))
		for i, item := range series.TimeSeries {
			floatSeries[i] = item.Value
		}

		result, err := j.changePointDetector.DetectChanges(ctx, floatSeries)

		var changePoints []model.ChangePoint
		for _, pointIndex := range result {
			mapped := model.CreateChangePoint(pointIndex, series.Measurement, j.changePointDetector.Algorithm().Name(), j.changePointDetector.Algorithm().Version(), algorithmConfigurationToOptions(j.changePointDetector.Algorithm().Configuration()))
			changePoints = append(changePoints, mapped)
		}

		if err != nil {
			j.AddError(errors.Wrapf(err, "Unable to detect change points in time series %s", j.PerformanceResultId))
			return
		}
		mappedChangePoints[series.Measurement] = changePoints
	}

	j.AddError(model.ReplaceChangePoints(ctx, j.env, performanceData, mappedChangePoints))
	j.AddError(model.MarkPerformanceResultsAsAnalyzed(ctx, j.env, performanceData.PerformanceResultId))
}

func algorithmConfigurationToOptions(configurationValues []perf.AlgorithmConfigurationValue) []model.AlgorithmOption {
	var options []model.AlgorithmOption
	for _, v := range configurationValues {
		options = append(options, model.AlgorithmOption{
			Name:  v.Name,
			Value: v.Value,
		})
	}
	return options
}
