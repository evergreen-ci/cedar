package operations

import (
	"context"
	"os"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
	"github.com/mongodb/grip/send"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type serviceConf struct {
	numWorkers          int
	localQueue          bool
	interactive         bool
	mongodbURI          string
	bucket              string
	dbName              string
	queueName           string
	dbUser              string
	dbPwd               string
	disableLocalLogging bool
}

func (c *serviceConf) export() *cedar.Configuration {
	return &cedar.Configuration{
		BucketName:         c.bucket,
		QueueName:          c.queueName,
		DatabaseName:       c.dbName,
		MongoDBURI:         c.mongodbURI,
		DisableRemoteQueue: c.localQueue,
		NumWorkers:         c.numWorkers,
		DBUser:             c.dbUser,
		DBPwd:              c.dbPwd,
	}
}

func (c *serviceConf) getSenders(ctx context.Context, conf *model.CedarConfig) (send.Sender, error) {
	senders := []send.Sender{}

	if c.interactive {
		senders = append(senders, send.MakeNative())
	} else if !c.disableLocalLogging {
		sender, err := send.MakeDefaultSystem()
		if err != nil {
			return nil, errors.WithStack(err)
		}

		senders = append(senders, sender)
	}

	if conf.IsNil() {
		if len(senders) > 0 {
			return senders[0], nil
		} else {
			return nil, errors.Errorf("no sender configured")
		}
	}

	logLevel := grip.GetSender().Level()

	fallback, err := send.NewErrorLogger("cedar.error", logLevel)
	if err != nil {
		return nil, errors.Wrap(err, "configuring fallback error logger")
	}

	var sender send.Sender

	if conf.Splunk.Populated() {
		sender, err = send.NewSplunkLogger("cedar", conf.Splunk, logLevel)
		if err != nil {
			return nil, errors.Wrap(err, "building Splunk logger")
		}
		if err = sender.SetErrorHandler(send.ErrorHandlerFromSender(fallback)); err != nil {
			return nil, errors.Wrap(err, "configuring Splunk logger error handler")
		}

		opts := send.BufferedSenderOptions{
			FlushInterval: conf.LoggerConfig.BufferDuration,
			BufferSize:    conf.LoggerConfig.BufferCount,
		}
		if conf.LoggerConfig.UseAsync {
			sender, err = send.NewBufferedAsyncSender(ctx, sender, send.BufferedAsyncSenderOptions{
				BufferedSenderOptions: opts,
				IncomingBufferFactor:  conf.LoggerConfig.IncomingBufferFactor,
			})
			if err != nil {
				return nil, errors.Wrap(err, "building buffered async Splunk logger")
			}
		} else {
			sender, err = send.NewBufferedSender(ctx, sender, opts)
			if err != nil {
				return nil, errors.Wrap(err, "building buffered Splunk logger")
			}
		}

		if err = sender.SetErrorHandler(send.ErrorHandlerFromSender(fallback)); err != nil {
			return nil, errors.Wrap(err, "configuring buffered Splunk logger error handler")
		}
		senders = append(senders, sender)
	}

	if conf.Slack.Options != nil {
		if err = conf.Slack.Options.Validate(); err != nil {
			return nil, errors.Wrap(err, "invalid Slack configuration")
		}

		if conf.Slack.Token == "" || conf.Slack.Level == "" {
			return nil, errors.Wrap(err, "must specify Slack token and logging threshold")
		}

		lvl := send.LevelInfo{
			Default:   logLevel.Default,
			Threshold: level.FromString(conf.Slack.Level),
		}

		sender, err = send.NewSlackLogger(conf.Slack.Options, conf.Slack.Token, lvl)
		if err != nil {
			return nil, errors.Wrap(err, "constructing Slack alert logger")
		}
		if err = sender.SetErrorHandler(send.ErrorHandlerFromSender(fallback)); err != nil {
			return nil, errors.Wrap(err, "configuring error handler")
		}

		// TODO consider using a local queue to buffer
		// these messages
		bufferedSender, err := send.NewBufferedSender(ctx, sender, send.BufferedSenderOptions{
			FlushInterval: conf.LoggerConfig.BufferDuration,
			BufferSize:    conf.LoggerConfig.BufferCount,
		})
		if err != nil {
			return nil, errors.Wrap(err, "building buffered Slack logger")
		}
		senders = append(senders, bufferedSender)
	}

	return send.NewConfiguredMultiSender(senders...), nil
}

func (c *serviceConf) setup(ctx context.Context) error {
	env, err := cedar.NewEnvironment(ctx, "cedar-service", c.export())
	if err != nil {
		return errors.WithStack(err)
	}
	cedar.SetEnvironment(env)

	conf := &model.CedarConfig{}
	conf.Setup(env)
	grip.Warning(conf.Find())

	sender, err := c.getSenders(ctx, conf)
	if err != nil {
		return errors.WithStack(err)
	}

	if err = grip.SetSender(sender); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

type dbCreds struct {
	DBUser string `yaml:"mdb_database_username"`
	DBPwd  string `yaml:"mdb_database_password"`
}

func loadCredsFromYAML(filePath string) (*dbCreds, error) {
	creds := &dbCreds{}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)

	if err := decoder.Decode(&creds); err != nil {
		return nil, err
	}

	return creds, nil
}

func newServiceConf(numWorkers int, localQueue bool, mongodbURI, bucket, dbName string, dbCredFile string, disableLocalLogging bool) *serviceConf {

	creds := &dbCreds{}
	var err error
	if dbCredFile != "" {
		creds, err = loadCredsFromYAML(dbCredFile)
		grip.Error(err)
	}

	return &serviceConf{
		numWorkers:          numWorkers,
		localQueue:          localQueue,
		mongodbURI:          mongodbURI,
		bucket:              bucket,
		dbName:              dbName,
		dbUser:              creds.DBUser,
		dbPwd:               creds.DBPwd,
		disableLocalLogging: disableLocalLogging,
	}
}
