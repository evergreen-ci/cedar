package units

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/pail"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

const (
	saveSimpleLogJobName = "simple-log-save"
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
	env       cedar.Environment
}

func saveSimpleLogToDBJobFactory() amboy.Job {
	j := &saveSimpleLogToDBJob{
		Base: &job.Base{
			JobType: amboy.JobType{
				Name:    saveSimpleLogJobName,
				Version: 1,
			},
		},
		env: cedar.GetEnvironment(),
	}
	j.SetDependency(dependency.NewAlways())

	return j
}

func MakeSaveSimpleLogJob(env cedar.Environment, logID, content string, ts time.Time, inc int) amboy.Job {
	j := saveSimpleLogToDBJobFactory().(*saveSimpleLogToDBJob)

	j.SetID(fmt.Sprintf("%s-%s-%d", j.Type().Name, logID, inc))

	j.Timestamp = ts
	j.Content = append(j.Content, content)
	j.LogID = logID
	j.Increment = inc
	j.env = env
	return j
}

func (j *saveSimpleLogToDBJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}

	conf := j.env.GetConf()

	bucket, err := pail.NewS3Bucket(pail.S3Options{Name: conf.BucketName})
	if err != nil {
		j.AddError(errors.WithStack(err))
		return
	}
	grip.Infoln("got s3 bucket object for:", conf.BucketName)

	s3Key := fmt.Sprintf("simple-log/%s.%d", j.LogID, j.Increment)

	writer, err := bucket.Writer(ctx, s3Key)
	if err != nil {
		j.AddError(errors.Wrap(err, "problem constructing bucket object"))
		return
	}

	_, err = writer.Write([]byte(strings.Join(j.Content, "\n")))
	if err != nil {
		j.AddError(errors.Wrap(err, "problem writing to s3"))
		return
	}
	if err = writer.Close(); err != nil {
		j.AddError(errors.Wrap(err, "problem flushing data to s3"))
	}

	// if we get here the data is safe in s3 so we can clear this.
	defer func() { j.Content = []string{} }()

	// in a simple log the log id and the id are different
	doc := &model.LogSegment{
		LogID:   j.LogID,
		Segment: j.Increment,
		URL:     fmt.Sprintf("http://%s.s3.amazonaws.com/%s", conf.BucketName, s3Key),
		Bucket:  conf.BucketName,
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

	// TODO: I think this needs to get data out of s3 rather than
	// get handed to it from memory, which requires a DB round trip.
	//
	parser := &parseSimpleLog{Key: j.LogID, Segment: j.Increment, Content: j.Content}

	// TODO: in the future there should be a parser/post-processing
	// interface that's amboy.Job+Validate(), so we can generate
	// slices of parsers and run the validate->put steps in a loop.
	//
	if err = parser.Validate(); err != nil {
		err = errors.Wrap(err, "problem creating parser job")
		grip.Error(err)
		j.AddError(err)
		return
	}

	q := j.env.GetLocalQueue()

	if err := q.Put(ctx, parser); err != nil {
		grip.Error(err)
		j.AddError(err)
		return
	}

	grip.Noticeln("added parsing job for:", j.LogID)
}
