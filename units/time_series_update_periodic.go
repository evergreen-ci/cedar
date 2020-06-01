package units

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
)

const periodicTimeSeriesUpdateJobName = "periodic-time-series-update"

type periodicTimeSeriesJob struct {
	*job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
	env       cedar.Environment
	queue     amboy.Queue
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
	j.SetDependency(dependency.NewAlways())
	return j
}

func NewPeriodicTimeSeriesUpdateJob(id string) amboy.Job {
	j := makePeriodicTimeSeriesUpdateJob()
	j.SetID(fmt.Sprintf("%s.%s", periodicTimeSeriesUpdateJobName, id))
	return j
}

func (j *periodicTimeSeriesJob) Run(ctx context.Context) {

	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}
	if j.queue == nil {
		j.queue = j.env.GetRemoteQueue()
	}
	defer j.MarkComplete()
	grip.Info("Scanning for performance data in need of analysis")
	needUpdates, err := model.GetPerformanceResultSeriesIdsNeedingTimeSeriesUpdate(ctx, j.env)
	if err != nil {
		j.AddError(errors.Wrap(err, "Unable to get metrics needing time series update"))
	}
	for _, id := range needUpdates {
		err := amboy.EnqueueUniqueJob(ctx, j.queue, NewUpdateTimeSeriesJob(id))
		if err != nil {
			j.AddError(err)
		}
	}
}
