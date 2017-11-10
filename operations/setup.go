package operations

import (
	"fmt"
	"time"

	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/cost"
	"github.com/evergreen-ci/sink/model"
	"github.com/evergreen-ci/sink/units"
	"github.com/mongodb/amboy"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/send"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

func configure(env sink.Environment, numWorkers int, localQueue bool, mongodbURI, bucket, dbName string) error {
	err := env.Configure(&sink.Configuration{
		BucketName:    bucket,
		DatabaseName:  dbName,
		MongoDBURI:    mongodbURI,
		UseLocalQueue: localQueue,
		NumWorkers:    numWorkers,
	})
	if err != nil {
		return errors.Wrap(err, "problem setting up configuration")
	}

	sender, err := model.NewDBSender(env, "sink")
	if err != nil {
		return errors.Wrapf(err, "problem creating system sender")
	}

	if err := env.GetSystemLogger().SetSender(sender); err != nil {
		return errors.Wrap(err, "problem setting system sender")
	}

	appConf := &model.SinkConfig{}
	appConf.Setup(env)
	if err := appConf.Find(); err != nil {
		return errors.Wrap(err, "problem fetching configuration from the database")
	}
	if !appConf.IsNil() {
		if appConf.Splunk.Populated() {
			splunkSender, err := send.NewSplunkLogger("sink", appConf.Splunk, grip.GetSender().Level())
			if err != nil {
				return errors.Wrap(err, "problem building plunk logger")
			}
			if err = env.getSplunkLogger().SetSender(splunkSender); err != nil {
				return errors.Wrap(err, "problem configuring splunk logger")
			}
		}
	}

	return nil
}

func backgroundJobs(ctx context.Context, env sink.Environment) error {
	// TODO: develop a specification format, either here or in
	// amboy so that you can specify a list of amboy.QueueOperation
	// functions + specific intervals
	//
	// In the mean time, we'll just register intervals here, and
	// hard code the configuration

	q, err := env.GetQueue()
	if err != nil {
		return errors.Wrap(err, "problem fetching queue")
	}

	// This isn't how we'd do this in the long term, but I want to
	// have one job running on an interval
	var count int

	amboy.PeriodicQueueOperation(ctx, q, time.Minute, true, func(cue amboy.Queue) error {
		name := "periodic-poc"
		count++
		j := units.NewHelloWorldJob(name)
		err = cue.Put(j)
		grip.Error(message.NewErrorWrap(err,
			"problem scheduling job %s (count: %d)", name, count))

		return err
	})

	amboy.IntervalQueueOperation(ctx, q, 15*time.Minute, time.Now(), true, func(cue amboy.Queue) error {
		now := time.Now().Add(-time.Hour)

		opts := &cost.EvergreenReportOptions{
			StartAt:  time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, time.UTC),
			Duration: time.Hour,
		}

		id := fmt.Sprintf("bcr-%s", opts.StartAt)

		j := units.NewBuildCostReport(env, id, opts)
		err := cue.Put(j)
		grip.Warning(err)

		return nil
	})

	return nil
}
