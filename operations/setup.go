package operations

import (
	"context"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/util"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
	"github.com/mongodb/grip/send"
	"github.com/pkg/errors"
)

const (
	loggingBufferCount    = 100
	loggingBufferDuration = 20 * time.Second
)

type serviceConf struct {
	numWorkers  int
	localQueue  bool
	interactive bool
	mongodbURI  string
	bucket      string
	dbName      string
	queueName   string
}

func (c *serviceConf) export() *cedar.Configuration {
	return &cedar.Configuration{
		BucketName:        c.bucket,
		QueueDatabaseName: c.queueName,
		DatabaseName:      c.dbName,
		MongoDBURI:        c.mongodbURI,
		UseLocalQueue:     c.localQueue,
		NumWorkers:        c.numWorkers,
	}
}

func (c *serviceConf) getSenders(conf *model.CedarConfig) (send.Sender, error) {
	senders := []send.Sender{}

	if c.interactive {
		senders = append(senders, send.MakeNative())
	} else {
		senders = append(senders, util.GetSystemSender())
	}

	if conf.IsNil() {
		return senders[0], nil
	}

	logLevel := grip.GetSender().Level()

	fallback, err := send.NewErrorLogger("cedar.error", logLevel)
	if err != nil {
		return nil, errors.Wrap(err, "problem configuring err fallback logger")
	}

	var sender send.Sender

	if conf.Splunk.Populated() {
		sender, err = send.NewSplunkLogger("cedar", conf.Splunk, logLevel)
		if err != nil {
			return nil, errors.Wrap(err, "problem building plunk logger")
		}
		if err = sender.SetErrorHandler(send.ErrorHandlerFromSender(fallback)); err != nil {
			return nil, errors.Wrap(err, "problem configuring error handler")
		}

		senders = append(senders, send.NewBufferedSender(sender, loggingBufferDuration, loggingBufferCount))
	}

	if conf.Slack.Options != nil {
		if err = conf.Slack.Options.Validate(); err != nil {
			return nil, errors.Wrap(err, "non-nil slack configuration is not valid")
		}

		if conf.Slack.Token == "" || conf.Slack.Level == "" {
			return nil, errors.Wrap(err, "must specify slack token and threshold")
		}

		lvl := send.LevelInfo{
			Default:   logLevel.Default,
			Threshold: level.FromString(conf.Slack.Level),
		}

		sender, err = send.NewSlackLogger(conf.Slack.Options, conf.Slack.Token, lvl)
		if err != nil {
			return nil, errors.Wrap(err, "problem constructing slack alert logger")
		}
		if err = sender.SetErrorHandler(send.ErrorHandlerFromSender(fallback)); err != nil {
			return nil, errors.Wrap(err, "problem configuring error handler")
		}

		// TODO consider using a local queue to buffer
		// these messages
		senders = append(senders, send.NewBufferedSender(sender, loggingBufferDuration, loggingBufferCount))
	}

	return send.NewConfiguredMultiSender(senders...), nil
}

func (c *serviceConf) setup(ctx context.Context, env cedar.Environment) error {
	err := env.Configure(c.export())
	if err != nil {
		return errors.Wrap(err, "problem setting up configuration")
	}

	conf := &model.CedarConfig{}
	conf.Setup(env)
	grip.Warning(conf.Find())

	sender, err := c.getSenders(conf)
	if err != nil {
		return errors.WithStack(err)
	}

	if err = grip.SetSender(sender); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func newServiceConf(numWorkers int, localQueue bool, mongodbURI, bucket, dbName string) *serviceConf {
	return &serviceConf{
		numWorkers: numWorkers,
		localQueue: localQueue,
		mongodbURI: mongodbURI,
		bucket:     bucket,
		dbName:     dbName,
	}
}
