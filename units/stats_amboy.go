package units

import (
	"context"
	"fmt"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/amboy/reporting"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

const (
	amboyStatsCollectorJobName = "amboy-stats-collector"
	enableExtendedRemoteStats  = false
)

func init() {
	registry.AddJobType(amboyStatsCollectorJobName,
		func() amboy.Job { return makeAmboyStatsCollector() })
}

type amboyStatsCollector struct {
	ExcludeLocal  bool `bson:"exclude_local" json:"exclude_local" yaml:"exclude_local"`
	ExcludeRemote bool `bson:"exclude_remote" json:"exclude_remote" yaml:"exclude_remote"`
	job.Base      `bson:"job_base" json:"job_base" yaml:"job_base"`
	env           cedar.Environment
}

// NewLocalAmboyStatsCollector reports the status of only the local queue
// registered in the evergreen service Environment.
func NewLocalAmboyStatsCollector(env cedar.Environment, id string) amboy.Job {
	j := makeAmboyStatsCollector()
	j.ExcludeRemote = true
	j.env = env
	j.SetID(fmt.Sprintf("%s-%s", amboyStatsCollectorJobName, id))
	return j
}

// NewRemoteAmboyStatsCollector reports the status of only the remote queue
// registered in the evergreen service Environment.
func NewRemoteAmboyStatsCollector(env cedar.Environment, id string) amboy.Job {
	j := makeAmboyStatsCollector()
	j.ExcludeLocal = true
	j.env = env
	j.SetID(fmt.Sprintf("%s-%s", amboyStatsCollectorJobName, id))
	return j
}

func makeAmboyStatsCollector() *amboyStatsCollector {
	j := &amboyStatsCollector{
		env: cedar.GetEnvironment(),
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    amboyStatsCollectorJobName,
				Version: 0,
			},
		},
	}

	j.SetDependency(dependency.NewAlways())
	return j
}

func (j *amboyStatsCollector) Run(ctx context.Context) {
	defer j.MarkComplete()

	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}

	localQueue := j.env.GetLocalQueue()
	remoteQueue := j.env.GetRemoteQueue()

	if !j.ExcludeLocal && (localQueue != nil && localQueue.Started()) {
		grip.Info(message.Fields{
			"message": "amboy local queue stats",
			"stats":   localQueue.Stats(),
		})
	}

	if !j.ExcludeRemote && (remoteQueue != nil && remoteQueue.Started()) {
		grip.Info(message.Fields{
			"message": "amboy remote queue stats",
			"stats":   remoteQueue.Stats(),
		})

		if enableExtendedRemoteStats {
			j.AddError(j.collectExtendedRemoteStats(ctx))
		}
	}
}

func (j *amboyStatsCollector) collectExtendedRemoteStats(ctx context.Context) error {
	reporter := j.env.GetRemoteReporter()
	if reporter == nil {
		return errors.New("reporter is not defined")
	}

	r := message.Fields{
		"message": "amboy remote queue report",
	}

	pending, err := reporter.JobStatus(ctx, reporting.Pending)
	j.AddError(err)
	if pending != nil {
		r["pending"] = pending
	}
	inprog, err := reporter.JobStatus(ctx, reporting.InProgress)
	j.AddError(err)
	if inprog != nil {
		r["inprog"] = inprog
	}
	stale, err := reporter.JobStatus(ctx, reporting.Stale)
	j.AddError(err)
	if stale != nil {
		r["stale"] = stale
	}

	recentErrors, err := reporter.RecentErrors(ctx, time.Minute, reporting.StatsOnly)
	j.AddError(err)
	if recentErrors != nil {
		r["errors"] = recentErrors
	}

	grip.InfoWhen(len(r) > 1, r)
	return nil
}
