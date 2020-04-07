package units

import (
	"context"
	"fmt"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/certdepot"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

const (
	serverCertRotationJobName = "server-cert-rotation"
)

type serverCertRotationJob struct {
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
	env      cedar.Environment
}

func init() {
	registry.AddJobType(serverCertRotationJobName, func() amboy.Job { return makeServerCertRotationJob() })
}

func makeServerCertRotationJob() *serverCertRotationJob {
	j := &serverCertRotationJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    serverCertRotationJobName,
				Version: 1,
			},
		},
	}

	j.SetDependency(dependency.NewAlways())
	return j
}

func NewServerCertRotationJob() amboy.Job {
	j := makeServerCertRotationJob()

	timestamp := utility.RoundPartOfDay(0)
	if timestamp.Day()%2 == 1 {
		timestamp.Add(-24 * time.Hour)
	}

	j.SetID(fmt.Sprintf("server-cert-rotation.%s", timestamp))

	return j
}

func (j *serverCertRotationJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}

	conf := model.NewCedarConfig(j.env)
	if err := conf.Find(); err != nil {
		err = errors.Wrap(err, "problem fetching application configuration")
		j.AddError(err)
		return
	}

	d, err := certdepot.CreateDepot(ctx, j.env.GetClient(), conf.CA.CertDepot)
	if err != nil {
		err = errors.Wrap(err, "problem creating depot")
		j.AddError(err)
		return
	}

	opts := conf.CA.CertDepot.ServiceOpts
	if opts == nil {
		opts = &certdepot.CertificateOptions{
			CA:         conf.CA.CertDepot.CAName,
			CommonName: conf.CA.CertDepot.ServiceName,
			Host:       conf.CA.CertDepot.ServiceName,
			Expires:    90 * 24 * time.Hour,
		}
	}
	created, err := opts.CreateCertificateOnExpiration(d, 7*24*time.Hour)
	if err != nil {
		err = errors.Wrap(err, "problem updating server certificate")
		j.AddError(err)
		return
	}

	if created {
		conf.CA.ServerCertVersion += 1
		if err = conf.Save(); err != nil {
			err = errors.Wrap(err, "problem saving configuration")
			j.AddError(err)
		}
		grip.Info("rotated server certificate")
	}
}
