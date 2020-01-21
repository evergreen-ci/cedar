package units

import (
	"context"
	"fmt"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
)

const periodicChangePointJobName = "periodic-change-point-detection"

type periodicChangePointJob struct {
	*job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
	env       cedar.Environment
	queue     amboy.Queue
}

func init() {
	registry.AddJobType(periodicChangePointJobName, func() amboy.Job { return makePeriodicChangePointJob() })
}

func makePeriodicChangePointJob() *periodicChangePointJob {
	j := &periodicChangePointJob{
		Base: &job.Base{
			JobType: amboy.JobType{
				Name:    periodicChangePointJobName,
				Version: 1,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func NewPeriodicChangePointJob(id string) amboy.Job {
	j := makePeriodicChangePointJob()
	j.SetID(fmt.Sprintf("%s.%s", periodicChangePointJobName, id))
	return j
}

func (j *periodicChangePointJob) Run(ctx context.Context) {

	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}
	if j.queue == nil {
		j.queue = j.env.GetRemoteQueue()
	}
	defer j.MarkComplete()
	grip.Info("Scanning for performance data in need of analysis")
	needUpdates, err := model.GetPerformanceResultSeriesIdsNeedingChangePointDetection(ctx, j.env)
	if err != nil {
		j.AddError(errors.Wrap(err, "Unable to get metrics needing change point detection"))
	}
	for _, id := range needUpdates {
		j.AddError(j.queue.Put(ctx, NewRecalculateChangePointsJob(id)))
	}
}
