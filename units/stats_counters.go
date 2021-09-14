package units

import (
	"context"
	"fmt"

	"github.com/evergreen-ci/cedar/model"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
)

const (
	statsCacheLoggerJobName = "stats-cache-logger"
)

func init() {
	registry.AddJobType(statsCacheLoggerJobName,
		func() amboy.Job { return makeStatsCacheLogger() })
}

type statsCacheLoggerJob struct {
	job.Base `bson:"job_base" json:"job_base" yaml:"job_base"`
}

// NewLocalAmboyStatsCollector reports the status of only the local queue
// registered in the evergreen service Environment.
func NewStatsCacheLogger(id string) amboy.Job {
	j := makeStatsCacheLogger()
	j.SetID(fmt.Sprintf("%s-%s", statsCacheLoggerJobName, id))
	return j
}

func makeStatsCacheLogger() *statsCacheLoggerJob {
	j := &statsCacheLoggerJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    statsCacheLoggerJobName,
				Version: 0,
			},
		},
	}

	j.SetDependency(dependency.NewAlways())
	return j
}

func (j *statsCacheLoggerJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	for _, counter := range model.CacheRegistry {
		counter.LogStats()
	}
}
