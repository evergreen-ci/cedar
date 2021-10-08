package units

import (
	"context"
	"fmt"

	"github.com/evergreen-ci/cedar"
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
	env      cedar.Environment
}

// NewStatsCacheLogger logs stats for each of the stats caches
// in the cache registry
func NewStatsCacheLogger(env cedar.Environment, id string) amboy.Job {
	j := makeStatsCacheLogger()
	j.SetID(fmt.Sprintf("%s-%s", statsCacheLoggerJobName, id))
	j.env = env
	return j
}

func makeStatsCacheLogger() *statsCacheLoggerJob {
	j := &statsCacheLoggerJob{
		env: cedar.GetEnvironment(),
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

	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}

	for _, name := range cedar.StatsCacheNames {
		counter := j.env.GetStatsCache(name)
		counter.LogStats()
	}

}
