package units

import (
	"context"
	"fmt"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/perf"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/pkg/errors"
)

const (
	findOutdatedRollupsJobName = "find-outdated-rollups"
	maxOutdatedPerfResults     = 1000
)

type findOutdatedRollupsJob struct {
	RollupTypes []string `bson:"rollup_types" json:"rollup_types" yaml:"rollup_types"`

	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
	env      cedar.Environment
	queue    amboy.Queue
	seenIDs  map[string]bool
}

func init() {
	registry.AddJobType(findOutdatedRollupsJobName, func() amboy.Job { return makeFindOutdatedRollupsJob() })
}

func makeFindOutdatedRollupsJob() *findOutdatedRollupsJob {
	j := &findOutdatedRollupsJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    findOutdatedRollupsJobName,
				Version: 1,
			},
		},
	}
	return j
}

func (j *findOutdatedRollupsJob) validate() error {
	if len(j.RollupTypes) == 0 {
		return errors.New("no rollup factories given")
	}

	return nil
}

func NewFindOutdatedRollupsJob(factories []perf.RollupFactory) (amboy.Job, error) {
	j := makeFindOutdatedRollupsJob()

	j.RollupTypes = []string{}
	for _, factory := range factories {
		j.RollupTypes = append(j.RollupTypes, factory.Type())
	}

	if err := j.validate(); err != nil {
		return nil, errors.Wrap(err, "creating new FTDC rollups job")
	}

	j.SetID(fmt.Sprintf("find-outdated-rollups.%s", time.Now()))

	return j, nil
}

func (j *findOutdatedRollupsJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}
	if j.queue == nil {
		j.queue = j.env.GetRemoteQueue()
	}
	if j.seenIDs == nil {
		j.seenIDs = map[string]bool{}
	}

	factories := []perf.RollupFactory{}
	for _, t := range j.RollupTypes {
		factory := perf.RollupFactoryFromType(t)
		if factory == nil {
			j.AddError(errors.Errorf("resolving rollup factory type '%s'", t))
			continue
		}
		factories = append(factories, factory)
	}

	count := 0
	results := model.PerformanceResults{}
	results.Setup(j.env)
	for i, factory := range factories {
		for _, name := range factory.Names() {
			after := time.Now().Add(-90 * 24 * time.Hour)
			if err := results.FindOutdatedRollups(ctx, name, factory.Version(), after, 3); err != nil {
				j.AddError(errors.Wrapf(err, "checking for outdated rollups for '%s'", name))
				continue
			}

			for _, result := range results.Results {
				if _, ok := j.seenIDs[result.ID]; !ok {
					j.createFTDCRollupsJobs(ctx, factories[i:], result)

					count += 1
					if count >= maxOutdatedPerfResults {
						// Stop job after processing a
						// max number of results to
						// avoid overwhelming the amboy
						// queue.
						return
					}
				}
			}
		}
	}
}

func (j *findOutdatedRollupsJob) createFTDCRollupsJobs(ctx context.Context, factories []perf.RollupFactory, result model.PerformanceResult) {
	outdated := findOutdatedFromResult(factories, result)

	job, err := NewFTDCRollupsJob(result.ID, getRawEventsArtifact(result.Artifacts), outdated, false)
	if err != nil {
		j.AddError(errors.Wrapf(err, "creating FTDC rollups job for performance result '%s'", result.ID))
		return
	}

	if err = j.queue.Put(ctx, job); err != nil {
		j.AddError(errors.Wrapf(err, "putting FTDC rollups job '%s' in remote queue", j.ID()))
		return
	}

	j.seenIDs[result.ID] = true
}

func getRawEventsArtifact(artifacts []model.ArtifactInfo) *model.ArtifactInfo {
	for _, artifact := range artifacts {
		if artifact.Schema == model.SchemaRawEvents {
			return &artifact
		}
	}

	return nil
}

func findOutdatedFromResult(factories []perf.RollupFactory, result model.PerformanceResult) []perf.RollupFactory {
	outdated := []perf.RollupFactory{}
	rollups := map[string]int{}
	for _, rollup := range result.Rollups.Stats {
		rollups[rollup.Name] = rollup.Version
	}

	for _, factory := range factories {
		for _, name := range factory.Names() {
			version, ok := rollups[name]
			if !ok {
				outdated = append(outdated, factory)
			} else if version < factory.Version() {
				outdated = append(outdated, factory)
			}
		}
	}

	return outdated
}
