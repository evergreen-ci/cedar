package units

import (
	"fmt"
	"strings"
	"time"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/curator/sthree"
	"github.com/pkg/errors"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/sink"
	"github.com/tychoish/sink/model"
	"github.com/tychoish/sink/parser"
)

const (
	saveSimpleLogJobName = "save-simple-log"
)

func init() {
	registry.AddJobType(saveSimpleLogJobName, func() amboy.Job {
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
				Name:    saveSimpleLogJobName,
				Version: 1,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())

	return j
}

func MakeSaveSimpleLogJob(logID, content string, ts time.Time, inc int) amboy.Job {
	j := saveSimpleLogToDBJobFactory().(*saveSimpleLogToDBJob)
	j.SetID(fmt.Sprintf("%s-%s-%d", j.Type().Name, logID, inc))

	j.Timestamp = ts
	j.Content = append(j.Content, content)
	j.LogID = logID
	j.Increment = inc

	return j
}
func (j *saveSimpleLogToDBJob) clear() { j.Content = []string{} }

func (j *saveSimpleLogToDBJob) Run() {
	defer j.MarkComplete()
	defer j.clear()

	conf := sink.GetConf()

	bucket := sthree.GetBucket(conf.BucketName)
	grip.Infoln("got s3 bucket object for:", bucket)

	s3Key := fmt.Sprintf("simple-log/%s.%d", j.LogID, j.Increment)
	err := bucket.Write([]byte(strings.Join(j.Content, "\n")), s3Key, "")
	if err != nil {
		j.AddError(errors.Wrap(err, "problem writing to s3"))
		return
	}

	// in a simple log the log id and the id are different
	doc := &model.Log{
		LogID:   j.LogID,
		Segment: j.Increment,
		URL:     fmt.Sprintf("http://s3.amazonaws.com/%s/%s", bucket, s3Key),
		Bucket:  bucket.String(),
		KeyName: s3Key,
		Metrics: model.LogMetrics{
			NumberLines:       -1,
			LetterFrequencies: map[string]int{},
		},
	}

	if err = doc.Insert(); err != nil {
		grip.Warning(message.Fields{"msg": "problem inserting document for log",
			"id":    doc.ID,
			"error": err,
			"doc":   fmt.Sprintf("%+v", doc)})
		j.AddError(errors.Wrap(err, "problem inserting record for document"))
		return
	}

	q, err := sink.GetQueue()
	if err != nil {
		j.AddError(errors.Wrap(err, "problem fetching queue"))
		return
	}

	// TODO: I think this needs to get data out of s3 rather than
	// get handed to it from memory.
	//
	opts := &parser.SimpleLog{Key: j.LogID, Content: j.Content}
	p, err := parser.MakeSimpleLogUnit(j.LogID, opts)
	if err != nil {
		grip.Error(err)
		j.AddError(err)
		return
	}

	if err := q.Put(p); err != nil {
		grip.Error(err)
		j.AddError(err)
		return
	}
	grip.Noticeln("added parsing job for:", j.LogID)
}
