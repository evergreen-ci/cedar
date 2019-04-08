package units

import (
	"context"
	"fmt"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/perf"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/ftdc"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

const (
	ftdcRollupsJobName = "ftdc-rollups"
)

type ftdcRollupsJob struct {
	PerfID       string              `bson:"perf_id" json:"perf_id" yaml:"perf_id"`
	ArtifactInfo *model.ArtifactInfo `bson:"artifact" json:"artifact" yaml:"artifact"`

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

	return nil
}

func NewFTDCRollupsJob(perfId string, artifactInfo *model.ArtifactInfo) (amboy.Job, error) {
	j := makeFTDCRollupsJob()
	j.PerfID = perfId
	j.ArtifactInfo = artifactInfo

	if err := j.validate(); err != nil {
		return nil, errors.Wrap(err, "failed to create new ftdc rollups job")
	}

	j.SetID(fmt.Sprintf("perf-rollup.%s.%s", perfId, artifactInfo.Path))

	return j, nil
}

func (j *ftdcRollupsJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}

	bucket, err := j.ArtifactInfo.Type.Create(j.env, j.ArtifactInfo.Bucket, j.ArtifactInfo.Prefix)
	if err != nil {
		grip.Warning(err)
		j.AddError(err)
		return
	}

	data, err := bucket.Get(ctx, j.ArtifactInfo.Path)
	if err != nil {
		err = errors.Wrap(err, "problem resolving bucket")
		grip.Warning(err)
		j.AddError(err)
		return
	}
	iter := ftdc.ReadChunks(ctx, data)

	stats, err := perf.CalculateDefaultRollups(iter)
	if err != nil {
		grip.Warning(err)
		j.AddError(err)
		return
	}

	result := &model.PerformanceResult{ID: j.PerfID}
	result.Setup(j.env)
	err = result.Find()
	if err != nil {
		j.AddError(errors.Wrap(err, "problem running query"))
		return
	}

	result.Rollups.Setup(j.env)
	for _, stat := range stats {
		err = result.Rollups.Add(ctx, stat.Name, stat.Version, stat.UserSubmitted, stat.MetricType, stat.Value)
		if err != nil {
			j.AddError(errors.Wrapf(err, "problem adding rollup %s", stat.Name))
		}
	}
}
