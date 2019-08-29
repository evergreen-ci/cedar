package units

import (
	"context"
	"fmt"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/perf"
	"github.com/evergreen-ci/cedar/util"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/ftdc"
	"github.com/pkg/errors"
)

const (
	ftdcRollupsJobName = "ftdc-rollups"
)

type ftdcRollupsJob struct {
	PerfID        string              `bson:"perf_id" json:"perf_id" yaml:"perf_id"`
	ArtifactInfo  *model.ArtifactInfo `bson:"artifact" json:"artifact" yaml:"artifact"`
	RollupTypes   []string            `bson:"rollup_types" json:"rollup_types" yaml:"rollup_types"`
	UserSubmitted bool                `bson:"user" json:"user" yaml:"user"`

	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
	env      cedar.Environment
}

func init() {
	registry.AddJobType(ftdcRollupsJobName, func() amboy.Job { return makeFTDCRollupsJob() })
}

func makeFTDCRollupsJob() *ftdcRollupsJob {
	j := &ftdcRollupsJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    ftdcRollupsJobName,
				Version: 1,
			},
		},
	}

	j.SetDependency(dependency.NewAlways())
	return j
}

func (j *ftdcRollupsJob) validate() error {
	if j.PerfID == "" {
		return errors.New("no id given")
	}

	if j.ArtifactInfo == nil {
		return errors.New("no artifact info given")
	}

	if len(j.RollupTypes) == 0 {
		return errors.New("no rollup factories given")
	}

	return nil
}

func NewFTDCRollupsJob(perfId string, artifactInfo *model.ArtifactInfo, factories []perf.RollupFactory, user bool) (amboy.Job, error) {
	j := makeFTDCRollupsJob()
	j.PerfID = perfId
	j.ArtifactInfo = artifactInfo
	j.UserSubmitted = user

	j.RollupTypes = []string{}
	for _, factory := range factories {
		j.RollupTypes = append(j.RollupTypes, factory.Type())
	}

	if err := j.validate(); err != nil {
		return nil, errors.Wrap(err, "failed to create new ftdc rollups job")
	}

	timestamp := util.RoundPartOfHour(0)
	if timestamp.Hour()%2 == 1 {
		timestamp.Add(-time.Hour)
	}

	j.SetID(fmt.Sprintf("perf-rollup.%s.%s.%s", perfId, artifactInfo.Path, timestamp))

	return j, nil
}

func (j *ftdcRollupsJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}

	bucket, err := j.ArtifactInfo.Type.Create(ctx, j.env, j.ArtifactInfo.Bucket, j.ArtifactInfo.Prefix, "")
	if err != nil {
		err = errors.Wrap(err, "problem resolving bucket")
		j.AddError(err)
		return
	}

	data, err := bucket.Get(ctx, j.ArtifactInfo.Path)
	if err != nil {
		err = errors.Wrap(err, "problem fetching artifact")
		j.AddError(err)
		return
	}
	iter := ftdc.ReadChunks(ctx, data)

	perfStats, err := perf.CreatePerformanceStats(iter)
	if err != nil {
		err = errors.Wrap(err, "problem computing performance statistics from raw data")
		j.AddError(err)
		return
	}

	rollups := []model.PerfRollupValue{}
	for _, t := range j.RollupTypes {
		factory := perf.RollupFactoryFromType(t)
		if factory == nil {
			err = errors.Errorf("problem resolving rollup factory type %s", t)
			j.AddError(err)
			continue
		}
		rollups = append(rollups, factory.Calc(perfStats, j.UserSubmitted)...)
	}

	result := &model.PerformanceResult{ID: j.PerfID}
	result.Setup(j.env)
	err = result.Find(ctx)
	if err != nil {
		err = errors.Wrap(err, "problem running query")
		j.AddError(err)
		return
	}

	result.Rollups.Setup(j.env)
	for _, r := range rollups {
		err = result.Rollups.Add(ctx, r)
		if err != nil {
			err = errors.Wrapf(err, "problem adding rollup %s for perf result %s", r.Name, j.PerfID)
			j.AddError(err)
		}
	}
}
