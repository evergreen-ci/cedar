package units

import (
	"fmt"
	"time"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/tychoish/grip"
)

type helloWorldJob struct {
	TaskID    string `bson:"task" json:"task" yaml:"task"`
	*job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

func init() {
	registry.AddJobType("hello-world", helloWorldJobFactory)
}

func helloWorldJobFactory() amboy.Job {
	j := &helloWorldJob{
		Base: &job.Base{
			JobType: amboy.JobType{
				Name:    "hello-world",
				Version: 1,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func NewHelloWorldJob(name string) amboy.Job {
	j := helloWorldJobFactory().(*helloWorldJob)
	j.SetID(fmt.Sprintf("%s-%d-%s-%s", j.Type().Name, job.GetNumber(), time.Now(), name))
	j.TaskID = name
	return j
}

func (j *helloWorldJob) Run() {
	defer j.MarkComplete()
	grip.Alertf("hello world job running, now: %s (%s)", j.ID(), j.TaskID)
}
