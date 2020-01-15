package units

import (
	"context"
	"fmt"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/util"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/pkg/errors"
)

type PeriodicChangePointJob struct {
	*job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
	env       cedar.Environment
	conf      *model.CedarConfig
	queue     amboy.Queue
}

func init() {
	registry.AddJobType("periodic-change-point-detection", func() amboy.Job { return makePeriodicChangePointJob() })
}

func makePeriodicChangePointJob() *PeriodicChangePointJob {
	j := &PeriodicChangePointJob{
		Base: &job.Base{
			JobType: amboy.JobType{
				Name:    "periodic-change-point-detection",
				Version: 1,
			},
		},
		env: cedar.GetEnvironment(),
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func NewPeriodicChangePointJob() amboy.Job {
	j := makePeriodicChangePointJob()
	// Every ten minutes at most
	timestamp := util.RoundPartOfHour(10)
	j.SetID(fmt.Sprintf("%s.%s", j.JobType.Name, timestamp))
	return j
}

func (j *PeriodicChangePointJob) Run(ctx context.Context) {
	if j.queue == nil {
		j.queue = j.env.GetRemoteQueue()
	}
	defer j.MarkComplete()
	if j.conf == nil {
		j.conf = model.NewCedarConfig(j.env)
	}
	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}
	needUpdates, err := model.GetPerformanceResultSeriesIdsNeedingChangePointDetection(ctx, j.env)
	if err != nil {
		j.AddError(errors.Wrapf(err, "Unable to get metrics needing change point detection"))
	}
	for _, id := range needUpdates {
		j.AddError(j.queue.Put(ctx, NewRecalculateChangePointsJob(id)))
	}
}
