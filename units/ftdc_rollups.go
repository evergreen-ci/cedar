package units

import (
	"context"

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

	// TODO: Do I need to set priority ?
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
	rollupJob := &ftdcRollupsJob{
		PerfID:       perfId,
		ArtifactInfo: artifactInfo,
	}

	if err := rollupJob.validate(); err != nil {
		return nil, errors.Wrap(err, "failed to create new ftdc rollups job")
	}
	return rollupJob, nil
}

func (j *ftdcRollupsJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	env := cedar.GetEnvironment()

	bucket, err := j.ArtifactInfo.Type.Create(env, j.ArtifactInfo.Bucket)
	if err != nil {
		grip.Warning(err)
		j.AddError(err)
		return
	}

	data, err := bucket.Get(ctx, j.ArtifactInfo.Path)
	if err != nil {
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
	result.Setup(env)
	err = result.Find()
	if err != nil {
		err = errors.Wrap(err, "problem running query")
		j.AddError(err)
		return
	}

	result.Rollups.Setup(env)
	for _, stat := range stats {
		err = result.Rollups.Add(stat.Name, stat.Version, stat.UserSubmitted, stat.MetricType, stat.Value)
		if err != nil {
			err = errors.Wrapf(err, "problem adding rollup %s", stat.Name)
			j.AddError(err)
		}
	}
}
