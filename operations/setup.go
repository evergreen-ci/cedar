package operations

import (
	"fmt"
	"time"

	"context"

	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/cost"
	"github.com/evergreen-ci/sink/model"
	"github.com/evergreen-ci/sink/units"
	"github.com/mongodb/amboy"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/send"
	"github.com/pkg/errors"
)

const (
	loggingBufferCount    = 100
	loggingBufferDuration = 20 * time.Second
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

	var fallback send.Sender
	fallback, err = send.NewErrorLogger("sink.error",
		send.LevelInfo{Default: level.Info, Threshold: level.Debug})
	if err != nil {
		return errors.Wrap(err, "problem configuring err fallback logger")
	}

	defaultSenders := []send.Sender{
		send.MakeNative(),
	}

	logLevelInfo := grip.GetSender().Level()

	appConf := &model.SinkConfig{}
	appConf.Setup(env)
	grip.Warning(appConf.Find())

	if !appConf.IsNil() {
		var sender send.Sender
		if appConf.Splunk.Populated() {
			sender, err = send.NewSplunkLogger("sink", appConf.Splunk, logLevelInfo)
			if err != nil {
				return errors.Wrap(err, "problem building plunk logger")
			}
			if err = sender.SetErrorHandler(send.ErrorHandlerFromSender(fallback)); err != nil {
				return errors.Wrap(err, "problem configuring error handler")
			}

			defaultSenders = append(defaultSenders, send.NewBufferedSender(sender, loggingBufferDuration, loggingBufferCount))
		}

		if appConf.Slack.Options != nil {
			sconf := appConf.Slack
			if err = sconf.Options.Validate(); err != nil {
				return errors.Wrap(err, "non-nil slack configuration is not valid")
			}

			if sconf.Token == "" || sconf.Level == "" {
				return errors.Wrap(err, "must specify slack token and threshold")
			}

			lvl := send.LevelInfo{
				Default:   logLevelInfo.Default,
				Threshold: level.FromString(sconf.Level),
			}

			sender, err = send.NewSlackLogger(sconf.Options, sconf.Token, lvl)
			if err != nil {
				return errors.Wrap(err, "problem constructing slack alert logger")
			}
			if err = sender.SetErrorHandler(send.ErrorHandlerFromSender(fallback)); err != nil {
				return errors.Wrap(err, "problem configuring error handler")
			}

			// TODO consider using a local queue to buffer
			// these messages
			defaultSenders = append(defaultSenders, send.NewBufferedSender(sender, loggingBufferDuration, loggingBufferCount))
		}
	}

	return errors.WithStack(grip.SetSender(send.NewConfiguredMultiSender(defaultSenders...)))
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

	conf := amboy.QueueOperationConfig{
		ContinueOnError: true,
	}

	amboy.PeriodicQueueOperation(ctx, q, time.Minute, conf, func(cue amboy.Queue) error {
		name := "periodic-poc"
		count++
		j := units.NewHelloWorldJob(name)
		err = cue.Put(j)
		grip.Error(message.NewErrorWrap(err,
			"problem scheduling job %s (count: %d)", name, count))

		return err
	})

	amboy.IntervalQueueOperation(ctx, q, 15*time.Minute, time.Now(), conf, func(cue amboy.Queue) error {
		now := time.Now().Add(-time.Hour)

		opts := &cost.EvergreenReportOptions{
			StartAt:  time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, time.UTC),
			Duration: time.Hour,
		}

		id := fmt.Sprintf("bcr-%s", opts.StartAt.Format(costReportDateFormat))

		j := units.NewBuildCostReport(env, id, opts)
		err := cue.Put(j)
		grip.Warning(err)

		return nil
	})

	return nil
}
