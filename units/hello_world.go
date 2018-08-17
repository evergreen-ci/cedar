package units

import (
	"context"
	"fmt"
	"time"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
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
	j.SetID(fmt.Sprintf("%s-%d-%s-%s", j.Type().Name, job.GetNumber(),
		time.Now().Format("2006-01-02::15.04.05"), name))
	j.TaskID = name
	return j
}

func (j *helloWorldJob) Run(_ context.Context) {
	defer j.MarkComplete()
	grip.Alertln("hello world job running, now:", j.ID())
}
