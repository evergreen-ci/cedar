package units

import (
	"context"
	"fmt"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/perf"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

const (
	findOutdatedRollupsJobName = "find-outdated-rollups"
)

type findOutdatedRollupsJob struct {
	RollupTypes []string `bson:"rollup_types" json:"rollup_types" yaml:rollup_types"`

	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
	env      cedar.Environment
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

	j.SetDependency(dependency.NewAlways())
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
		return nil, errors.Wrap(err, "failed to create new ftdc rollups job")
	}

	j.SetID(fmt.Sprintf("find-outdated-rollups.%s", time.Now()))

	return j, nil
}

func (j *findOutdatedRollupsJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}

	factories := []perf.RollupFactory{}
	for _, t := range j.RollupTypes {
		factory := perf.RollupFactoryFromType(t)
		if factory == nil {
			err := errors.Errorf("problem resolving rollup factory type %s", t)
			grip.Warning(err)
			j.AddError(err)
			continue
		}
		factories = append(factories, factory)
	}

	results := model.PerformanceResults{}
	results.Setup(j.env)
	seenIDs := map[string]bool{}
	for i, factory := range factories {
		for _, name := range factory.Names() {
			if err := results.FindOutdatedRollups(name, factory.Version()); err != nil {
				err = errors.Wrapf(err, "problem checking for outdated rollups for %s", name)
				grip.Warning(err)
				j.AddError(err)
				continue
			}

			for _, result := range results.Results {
				if _, ok := seenIDs[result.Info.ID()]; ok {
					continue
				}
				outdated := findOutdatedFromResult(factories[i+1:len(factories)], result)
				outdated = append(outdated, factory)

				job, err := NewFTDCRollupsJob(result.Info.ID(), getFTDCArtifact(result.Artifacts), outdated, false)
				if err != nil {
					err = errors.Wrapf(err, "problem creating FTDC rollups job for %s", result.Info.ID())
					grip.Warning(err)
					j.AddError(err)
					continue
				}

				if err = j.env.GetRemoteQueue().Put(job); err != nil {
					err = errors.Wrapf(err, "problem putting FTDC rollups job %s on remote queue", j.ID())
					grip.Warning(err)
					j.AddError(err)
					continue
				}

				seenIDs[result.Info.ID()] = true
			}
		}
	}
}

func getFTDCArtifact(artifacts []model.ArtifactInfo) *model.ArtifactInfo {
	for _, artifact := range artifacts {
		if artifact.Format == model.FileFTDC {
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
