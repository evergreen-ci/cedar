package units

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
)

const periodicTimeSeriesUpdateJobName = "periodic-time-series-update"

type periodicTimeSeriesJob struct {
	*job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`

	env   cedar.Environment
	queue amboy.Queue
}

func init() {
	registry.AddJobType(periodicTimeSeriesUpdateJobName, func() amboy.Job { return makePeriodicTimeSeriesUpdateJob() })
}

func makePeriodicTimeSeriesUpdateJob() *periodicTimeSeriesJob {
	j := &periodicTimeSeriesJob{
		Base: &job.Base{
			JobType: amboy.JobType{
				Name:    periodicTimeSeriesUpdateJobName,
				Version: 1,
			},
		},
	}
	return j
}

// NewPeriodicTimeSeriesUpdateJob creates a new amboy job to periodically update time series.
func NewPeriodicTimeSeriesUpdateJob(id string) amboy.Job {
	j := makePeriodicTimeSeriesUpdateJob()
	j.SetID(fmt.Sprintf("%s.%s", periodicTimeSeriesUpdateJobName, id))
	return j
}

func (j *periodicTimeSeriesJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}

	if j.queue == nil {
		j.queue = j.env.GetRemoteQueue()
	}

	outdatedSeries, err := model.GetUnanalyzedPerformanceSeries(ctx, j.env)
	if err != nil {
		j.AddError(errors.Wrap(err, "getting metrics needing time series update"))
		return
	}

	for _, series := range outdatedSeries {
		err := amboy.EnqueueUniqueJob(ctx, j.queue, NewUpdateTimeSeriesJob(series))
		if err != nil {
			j.AddError(err)
			return
		}
	}
}
