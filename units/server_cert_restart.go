package units

import (
	"context"
	"fmt"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/util"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

const (
	serverCertRestartJobName = "server-cert-restart"
)

type serverCertRestartJob struct {
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
	env      cedar.Environment
}

func init() {
	registry.AddJobType(serverCertRestartJobName, func() amboy.Job { return makeServerCertRestartJob() })
}

func makeServerCertRestartJob() *serverCertRestartJob {
	j := &serverCertRestartJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    serverCertRestartJobName,
				Version: 1,
			},
		},
	}

	j.SetDependency(dependency.NewAlways())
	return j
}

func NewServerCertRestartJob() amboy.Job {
	j := makeServerCertRestartJob()

	timestamp := util.RoundPartOfHour(0)
	if timestamp.Hour()%2 == 1 {
		timestamp.Add(-time.Hour)
	}

	j.SetID(fmt.Sprintf("server-cert-restart.%s", timestamp))

	return j
}

func (j *serverCertRestartJob) Run(ctx context.Context) {
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

	localServerCertVersion := j.env.GetServerCertVersion()
	if localServerCertVersion < 0 {
		j.env.SetServerCertVersion(conf.CA.ServerCertVersion)
	} else if localServerCertVersion < conf.CA.ServerCertVersion {
		grip.Info("restarting application to update server certificate")
		closeCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := j.env.Close(closeCtx); err != nil {
			err = errors.Wrap(err, "problem restarting application with an outdated server certificate")
			j.AddError(err)
			return
		}
	}
}
