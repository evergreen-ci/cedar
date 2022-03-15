package operations

import (
	"context"
	"os"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
	"github.com/mongodb/grip/send"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type serviceConf struct {
	numWorkers  int
	localQueue  bool
	interactive bool
	mongodbURI  string
	bucket      string
	dbName      string
	queueName   string
	dbUser      string
	dbPwd       string
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
	} else {
		sender, err := send.MakeDefaultSystem()
		if err != nil {
			return nil, errors.WithStack(err)
		}

		senders = append(senders, sender)
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
			return nil, errors.Wrap(err, "problem building splunk logger")
		}
		if err = sender.SetErrorHandler(send.ErrorHandlerFromSender(fallback)); err != nil {
			return nil, errors.Wrap(err, "configuring splunk logger error handler")
		}

		opts := send.BufferedSenderOptions{
			FlushInterval: conf.LoggerConfig.BufferDuration * time.Second,
			BufferSize:    conf.LoggerConfig.BufferCount,
		}
		if conf.LoggerConfig.UseAsync {
			sender, err = send.NewBufferedAsyncSender(ctx, sender, send.BufferedAsyncSenderOptions{BufferedSenderOptions: opts})
			if err != nil {
				return nil, errors.Wrap(err, "building buffered async splunk logger")
			}
		} else {
			sender, err = send.NewBufferedSender(ctx, sender, opts)
			if err != nil {
				return nil, errors.Wrap(err, "building buffered splunk logger")
			}
		}

		if err = sender.SetErrorHandler(send.ErrorHandlerFromSender(fallback)); err != nil {
			return nil, errors.Wrap(err, "configuring buffered splunk logger error handler")
		}
		senders = append(senders, sender)
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
		bufferedSender, err := send.NewBufferedSender(ctx, sender, send.BufferedSenderOptions{
			FlushInterval: conf.LoggerConfig.BufferDuration * time.Second,
			BufferSize:    conf.LoggerConfig.BufferCount,
		})
		if err != nil {
			return nil, errors.Wrap(err, "building buffered slack logger")
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

func newServiceConf(numWorkers int, localQueue bool, mongodbURI, bucket, dbName string, dbCredFile string) *serviceConf {

	creds := &dbCreds{}
	var err error
	if dbCredFile != "" {
		creds, err = loadCredsFromYAML(dbCredFile)
		grip.Error(err)
	}

	return &serviceConf{
		numWorkers: numWorkers,
		localQueue: localQueue,
		mongodbURI: mongodbURI,
		bucket:     bucket,
		dbName:     dbName,
		dbUser:     creds.DBUser,
		dbPwd:      creds.DBPwd,
	}
}
