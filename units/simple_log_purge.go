package units

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/model"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/curator/sthree"
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
	env       sink.Environment
}

func mergeSimpleLogJobFactory() amboy.Job {
	j := &mergeSimpleLogJob{
		Base: &job.Base{
			JobType: amboy.JobType{
				Name:    mergeSimpleLogJobName,
				Version: 1,
			},
		},
		env: sink.GetEnvironment(),
	}

	j.SetDependency(dependency.NewAlways())
	return j
}

func MakeMergeSimpleLogJob(env sink.Environment, logID string) amboy.Job {
	j := mergeSimpleLogJobFactory().(*mergeSimpleLogJob)
	j.SetID(fmt.Sprintf("%s-%s-%s", j.Type().Name, logID,
		time.Now().Format("2006-01-02.15")))

	j.LogID = logID
	j.env = env
	return j
}

func (j *mergeSimpleLogJob) Run(ctx context.Context) {
	logs := &model.LogSegments{}

	err := errors.Wrap(logs.Find(j.LogID, true),
		"problem running query for all logs of a segment")

	if err != nil {
		grip.Warning(err)
		j.AddError(err)
		return
	}

	record := &model.LogRecord{}

	if err = record.Find(j.LogID); err != nil {
		grip.Infof("no existing record for %s, creating...", j.LogID)

		prototypeLog := &model.LogSegment{}
		if err = prototypeLog.Find(j.LogID, -1); err != nil {
			err = errors.Wrapf(err, "problem finding a prototype log for %s", j.LogID)
			grip.Warning(err)
			j.AddError(err)
			return
		}
		record.LogID = j.LogID
		record.Bucket = prototypeLog.Bucket
		record.KeyName = fmt.Sprintf("simple-log/%s", j.LogID)

		if err = record.Insert(); err != nil {
			err = errors.Wrap(err, "problem inserting log record document")
			grip.Warning(err)
			j.AddError(err)
			return
		}

	}
	conf, err := j.env.GetConf()
	if err != nil {
		grip.Warning(err)
		j.AddError(err)
		return
	}

	bucket := sthree.GetBucket(conf.BucketName)

	buffer := bytes.NewBuffer([]byte{})
	segments := logs.Slice()
	var seg []byte
	for _, log := range segments {
		seg, err = bucket.Read(log.KeyName)
		if err != nil {
			err = errors.Wrapf(err, "problem reading segment %s from bucket %s",
				log.KeyName, bucket)
			grip.Critical(err)
			j.AddError(err)
			return
		}
		_, err = buffer.Write(seg)
		if err != nil {
			err = errors.Wrap(err, "problem writing data to buffer")
			j.AddError(err)
			return
		}

		if log.Segment > record.LastSegment {
			record.LastSegment = log.Segment
		}
	}

	err = errors.Wrap(bucket.Write(buffer.Bytes(), record.KeyName, ""),
		"problem writing merged data to s3")
	if err != nil {
		grip.Error(err)
		j.AddError(err)
		return
	}

	catcher := grip.NewCatcher()
	for _, log := range segments {
		err = errors.Wrap(bucket.Delete(log.KeyName), "problem deleting segment from logs")
		if err != nil {
			catcher.Add(err)
			j.AddError(err)
			continue
		}

		catcher.Add(log.Remove())
	}
	grip.Info(catcher.Resolve())

	err = errors.Wrapf(record.Insert(), "problem saving master log record for %s", j.LogID)
	if err != nil {
		grip.Critical(err)
		j.AddError(err)
		return
	}
}
