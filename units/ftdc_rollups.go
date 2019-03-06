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

type ftdcRollups struct {
	PerfID       string              `bson:"perf_id" json:"perf_id" yaml:"perf_id"`
	ArtifactInfo *model.ArtifactInfo `bson:"artifact" json:"artifact" yaml:"artifact"`

	*job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

func init() {
	registry.AddJobType(ftdcRollupsJobName, func() amboy.Job {
		fr := &ftdcRollups{}
		fr.setup()
		fr.SetDependency(dependency.NewAlways())

		return fr
	})
}

func (fr *ftdcRollups) setup() {
	if fr.Base == nil {
		fr.Base = &job.Base{
			JobType: amboy.JobType{
				Name:    ftdcRollupsJobName,
				Version: 1,
			},
		}
	}
}

func (fr *ftdcRollups) Validate() error {
	fr.setup()
	fr.SetDependency(dependency.NewAlways())

	if fr.PerfID == "" {
		return errors.New("no id given")
	}

	if fr.ArtifactInfo == nil {
		return errors.New("no artifact info given")
	}

	return nil
}

func (fr *ftdcRollups) reset() {
	fr.ArtifactInfo = nil
}

func NewFTDCRollupsJob(perfId string, artifactInfo *model.ArtifactInfo) (amboy.Job, error) {
	rollupJob := &ftdcRollups{
		PerfID:       perfId,
		ArtifactInfo: artifactInfo,
	}

	if err := rollupJob.Validate(); err != nil {
		return nil, errors.Wrap(err, "failed to setup new ftdc rollups job")
	}
	return rollupJob, nil
}

func (fr *ftdcRollups) Run(ctx context.Context) {
	defer fr.MarkComplete()
	defer fr.reset()

	env := cedar.GetEnvironment()

	bucket, err := fr.ArtifactInfo.Type.Create(env, fr.ArtifactInfo.Bucket)
	if err != nil {
		grip.Warning(err)
		fr.AddError(err)
		return
	}

	data, err := bucket.Get(ctx, fr.ArtifactInfo.Path)
	if err != nil {
		grip.Warning(err)
		fr.AddError(err)
		return
	}
	iter := ftdc.ReadChunks(ctx, data)

	stats, err := perf.CalculateDefaultRollups(iter)
	if err != nil {
		grip.Warning(err)
		fr.AddError(err)
		return
	}

	result := &model.PerformanceResult{ID: fr.PerfID}
	result.Setup(env)
	err = result.Find()
	if err != nil {
		err = errors.Wrap(err, "problem running query")
		fr.AddError(err)
		return
	}

	result.Rollups.Setup(env)
	for _, stat := range stats {
		err = result.Rollups.Add(stat.Name, stat.Version, stat.UserSubmitted, stat.MetricType, stat.Value)
		if err != nil {
			err = errors.Wrapf(err, "problem adding rollup %s", stat.Name)
			fr.AddError(err)
		}
	}
}
