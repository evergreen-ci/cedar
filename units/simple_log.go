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

func init() {
	registry.AddJobType("save-simple-log", func() amboy.Job {
		return saveSimpleLogToDBJobFactory()
	})
}

type saveSimpleLogToDBJob struct {
	Timestamp time.Time `bson:"ts" json:"ts" yaml:"timestamp"`
	Content   []string  `bson:"content" json:"content" yaml:"content"`
	Increment int       `bson:"i" json:"inc" yaml:"increment"`
	LogID     string    `bson:"logID" json:"logID" yaml:"logID"`
	*job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

func saveSimpleLogToDBJobFactory() amboy.Job {
	j := &saveSimpleLogToDBJob{
		Base: &job.Base{
			JobType: amboy.JobType{
				Name:    "save-simple-log",
				Version: 1,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())

	return j
}

func MakeSaveSimpleLogJob(logID, content string, ts time.Time, inc int) amboy.Job {
	j := saveSimpleLogToDBJobFactory().(*saveSimpleLogToDBJob)
	j.SetID(fmt.Sprintf("%s-%s-%d", j.Type(), logID, inc))

	j.Timestamp = ts
	j.Content = append(j.Content, content)
	j.LogID = logID
	j.Increment = inc

	return j
}

func (j *saveSimpleLogToDBJob) Run() {
	defer j.MarkComplete()

	grip.Alert("would save data to s3")
	grip.Alert("would save remove copy of content from this job here")
	grip.Alert("would submit multiple jobs to trigger post processing")

	return
}
