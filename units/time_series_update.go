package units

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/perf"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/sometimes"
	"github.com/pkg/errors"
)

type timeSeriesUpdateJob struct {
	Series    model.UnanalyzedPerformanceSeries `bson:"series" json:"series" yaml:"series"`
	*job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`

	env     cedar.Environment
	conf    *model.CedarConfig
	service perf.PerformanceAnalysisService
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
	return j
}

// NewUpdateTimeSeriesJob creates a new amboy job to update a time series.
func NewUpdateTimeSeriesJob(series model.UnanalyzedPerformanceSeries) amboy.Job {
	j := makeTimeSeriesJob()
	baseID := fmt.Sprintf(
		"%s.%s.%s.%s.%s.%s",
		j.JobType.Name,
		series.Project,
		series.Variant,
		series.Task,
		series.Test,
		series.Arguments,
	)
	j.SetID(fmt.Sprintf("%s.%s", baseID, time.Now().UTC()))
	j.SetScopes([]string{baseID})
	j.SetEnqueueAllScopes(true)
	j.Series = series
	return j
}

func (j *timeSeriesUpdateJob) makeMessage(msg string) message.Fields {
	return message.Fields{
		"job_id":    j.ID(),
		"message":   msg,
		"project":   j.Series.Project,
		"variant":   j.Series.Variant,
		"task_name": j.Series.Task,
		"test_name": j.Series.Test,
		"args":      j.Series.Arguments,
	}
}

func (j *timeSeriesUpdateJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}

	if j.conf == nil {
		j.conf = model.NewCedarConfig(j.env)
		err := j.conf.Find()
		if err != nil {
			j.AddError(errors.Wrap(err, "getting cedar configuration"))
			return
		}
	}

	baseID := j.Series.CreateBaseSeriesID()

	if j.conf.Flags.DisableSignalProcessing {
		grip.InfoWhen(sometimes.Percent(10), j.makeMessage("signal processing is disabled, skipping processing"))
		return
	}

	if j.service == nil {
		j.service = perf.NewPerformanceAnalysisService(j.conf.ChangeDetector.URI, j.conf.ChangeDetector.User, j.conf.ChangeDetector.Token)
	}

	for _, id := range j.Series.CreateSeriesIDs() {
		if err := j.service.ReportUpdatedTimeSeries(ctx, createTimeSeriesModel(id)); err != nil {
			j.AddError(errors.Wrapf(err, "updating time series for perf data '%s'", baseID))
			return
		}
	}

	j.AddError(model.MarkPerformanceResultsAsAnalyzed(ctx, j.env, baseID))
}

func createTimeSeriesModel(id model.PerformanceResultSeriesID) perf.TimeSeriesModel {
	series := perf.TimeSeriesModel{
		Project:     id.Project,
		Variant:     id.Variant,
		Task:        id.Task,
		Test:        id.Test,
		Measurement: id.Measurement,
	}
	series.Arguments = []perf.ArgumentsModel{}
	for k, v := range id.Arguments {
		series.Arguments = append(series.Arguments, perf.ArgumentsModel{
			Name:  k,
			Value: v,
		})
	}
	sort.Slice(series.Arguments, func(i, j int) bool {
		return series.Arguments[i].Name < series.Arguments[j].Name
	})

	return series
}
