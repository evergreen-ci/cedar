package units

import (
	"context"
	"fmt"
	"sort"

	"github.com/aclements/go-moremath/stats"
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
	timestamp := utility.RoundPartOfHour(10)
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
		j.AddError(errors.Wrapf(err, "Unable to aggregate time perfData %s", j.PerformanceResultId))
		return
	}
	if performanceData == nil {
		j.AddError(model.MarkPerformanceResultsAsAnalyzed(ctx, j.env, j.PerformanceResultId))
		return
	}
	mappedChangePoints := map[string][]model.ChangePoint{}
	for _, perfData := range performanceData.Data {
		sort.Slice(perfData.TimeSeries, func(i, j int) bool {
			return perfData.TimeSeries[i].Order < perfData.TimeSeries[j].Order
		})
		latestTriagedChangePointIndex := 0
		for _, cp := range perfData.ChangePoints {
			if cp.Index > latestTriagedChangePointIndex && cp.Triage.Status != model.TriageStatusUntriaged {
				latestTriagedChangePointIndex = cp.Index
			}
		}
		floatSeries := make([]float64, len(perfData.TimeSeries)-latestTriagedChangePointIndex)
		for i, item := range perfData.TimeSeries {
			if i >= latestTriagedChangePointIndex {
				floatSeries[i-latestTriagedChangePointIndex] = item.Value
			}
		}

		result, err := j.changePointDetector.DetectChanges(ctx, floatSeries)
		if err != nil {
			j.AddError(errors.Wrapf(err, "Unable to detect change points in time perfData %s", j.PerformanceResultId))
			return
		}

		var changePoints []model.ChangePoint
		for i, pointIndex := range result {
			var lowerBound int
			var upperBound int

			if i == 0 {
				lowerBound = 0
			} else {
				lowerBound = result[i-1]
			}

			if i == len(result)-1 {
				upperBound = len(floatSeries)
			} else {
				upperBound = result[i+1]
			}

			lowerWindow := floatSeries[lowerBound:pointIndex]
			upperWindow := floatSeries[pointIndex:upperBound]
			percentChange := calculatePercentChange(lowerWindow, upperWindow)

			mapped := model.CreateChangePoint(pointIndex+latestTriagedChangePointIndex, perfData.Measurement, j.changePointDetector.Algorithm().Name(), j.changePointDetector.Algorithm().Version(), algorithmConfigurationToOptions(j.changePointDetector.Algorithm().Configuration()), percentChange)
			changePoints = append(changePoints, mapped)
		}

		mappedChangePoints[perfData.Measurement] = changePoints
	}

	j.AddError(model.ReplaceChangePoints(ctx, j.env, performanceData, mappedChangePoints))
	j.AddError(model.MarkPerformanceResultsAsAnalyzed(ctx, j.env, performanceData.PerformanceResultId))
}

func calculatePercentChange(lowerWindow, upperWindow []float64) float64 {
	avgLowerWindow := stats.Mean(lowerWindow)
	avgUpperWindow := stats.Mean(upperWindow)

	return 100 * ((avgUpperWindow / avgLowerWindow) - 1)
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
