package units

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/pail"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

const (
	mergeSimpleLogJobName = "merge-simple-log"
)

func init() {
	registry.AddJobType(mergeSimpleLogJobName, func() amboy.Job {
		return mergeSimpleLogJobFactory()
	})
}

type mergeSimpleLogJob struct {
	LogID     string `bson:"logID" json:"logID" yaml:"logID"`
	*job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
	env       cedar.Environment
}

func mergeSimpleLogJobFactory() amboy.Job {
	j := &mergeSimpleLogJob{
		Base: &job.Base{
			JobType: amboy.JobType{
				Name:    mergeSimpleLogJobName,
				Version: 1,
			},
		},
		env: cedar.GetEnvironment(),
	}
	return j
}

func MakeMergeSimpleLogJob(env cedar.Environment, logID string) amboy.Job {
	j := mergeSimpleLogJobFactory().(*mergeSimpleLogJob)
	j.SetID(fmt.Sprintf("%s-%s-%s", j.Type().Name, logID,
		time.Now().Format("2006-01-02.15")))

	j.LogID = logID
	j.env = env
	return j
}

func (j *mergeSimpleLogJob) Run(ctx context.Context) {
	logs := &model.LogSegments{}

	err := errors.Wrap(logs.Find(j.LogID, true), "running query for all logs of a segment")

	if err != nil {
		grip.Warning(err)
		j.AddError(err)
		return
	}

	record := &model.LogRecord{
		LogID: j.LogID,
	}

	if err = record.Find(); err != nil {
		record.LogID = ""
		grip.Infof("no existing record for %s, creating...", j.LogID)

		prototypeLog := &model.LogSegment{}
		if err = prototypeLog.Find(j.LogID, -1); err != nil {
			err = errors.Wrapf(err, "finding a prototype log for '%s'", j.LogID)
			grip.Warning(err)
			j.AddError(err)
			return
		}
		record.LogID = j.LogID
		record.Bucket = prototypeLog.Bucket
		record.KeyName = fmt.Sprintf("simple-log/%s", j.LogID)

		if err = record.Save(); err != nil {
			err = errors.Wrap(err, "inserting log record document")
			grip.Warning(err)
			j.AddError(err)
			return
		}

	}
	conf := j.env.GetConfig()

	bucket, err := pail.NewS3Bucket(ctx, pail.S3Options{Name: conf.BucketName})
	if err != nil {
		j.AddError(errors.WithStack(err))
		return
	}

	buffer := bytes.NewBuffer([]byte{})
	segments := logs.Slice()

	var seg []byte
	var reader io.ReadCloser

	for _, log := range segments {
		reader, err = bucket.Reader(ctx, log.KeyName)
		if err != nil {
			j.AddError(errors.WithStack(err))
			continue
		}
		defer reader.Close()

		seg, err = ioutil.ReadAll(reader)
		if err != nil {
			j.AddError(errors.Wrapf(err, "reading segment '%s' from bucket '%s'", log.KeyName, bucket))
			continue
		}
		_, err = buffer.Write(seg)
		if err != nil {
			j.AddError(errors.Wrap(err, "writing data to buffer"))
			return
		}

		if log.Segment > record.LastSegment {
			record.LastSegment = log.Segment
		}
	}

	if err = errors.Wrap(bucket.Put(ctx, record.KeyName, buffer), "writing merged data to S3"); err != nil {
		j.AddError(err)
		return
	}

	for _, log := range segments {
		if err = errors.Wrap(bucket.Remove(ctx, log.KeyName), "deleting segment from logs"); err != nil {
			j.AddError(err)
			continue
		}

		j.AddError(log.Remove())
	}

	j.AddError(errors.Wrapf(record.Save(), "saving master log record for log ID '%s'", j.LogID))
}
