package operations

import (
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
	"github.com/mongodb/grip/send"
	"github.com/pkg/errors"
)

const (
	loggingBufferCount    = 100
	loggingBufferDuration = 20 * time.Second
)

func configure(env cedar.Environment, numWorkers int, localQueue bool, mongodbURI, bucket, dbName string) error {
	err := env.Configure(&cedar.Configuration{
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
	fallback, err = send.NewErrorLogger("cedar.error",
		send.LevelInfo{Default: level.Info, Threshold: level.Debug})
	if err != nil {
		return errors.Wrap(err, "problem configuring err fallback logger")
	}

	defaultSenders := []send.Sender{
		send.MakeNative(),
	}

	logLevelInfo := grip.GetSender().Level()

	appConf := &model.CedarConfig{}
	appConf.Setup(env)
	grip.Warning(appConf.Find())

	if !appConf.IsNil() {
		var sender send.Sender
		if appConf.Splunk.Populated() {
			sender, err = send.NewSplunkLogger("cedar", appConf.Splunk, logLevelInfo)
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
