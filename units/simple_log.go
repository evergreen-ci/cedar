package units

import (
	"fmt"
	"time"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/pkg/errors"
	"github.com/tychoish/grip"
	"github.com/tychoish/sink"
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
	// s3.SaveData(opts{bucket: bucketName; key:LogID, c.Content})

	grip.Alert("would save remove copy of content from this job here")
	// j.Content = []string{}

	grip.Alert("would submit multiple jobs to trigger post processing")
	q, err := sink.GetQueue()
	if err != nil {
		j.AddError(errors.Wrap(err, "problem fetching queue"))
		return
	}
	grip.Debug(q)

	session, _, err := sink.GetMgoSession()
	if err != nil {
		j.AddError(errors.Wrap(err, "problem fetching database connection"))
		return
	}
	grip.Debug(session)

	grip.Info("would create a document here in the db")
	// model.CreateLogRecord(session, j.LogID)

	// lots of the following jobs should happen here, should happen here:
	// if err := q.Put(MakeParserJobOne(j.LogID, j.Content)); err != nil {
	// 	j.AddError(err)
	// 	return
	// }

	// as an intermediary we could just do a lot of log parsing
	// here. to start with and then move that out to other jobs
	// later.
	//
	// eventually we'll want this to be in seperate jobs because
	// we'll want to add additional parsers and be able to
	// idempotently update our records/metadata for each log.
}
