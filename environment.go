package cedar

import (
	"context"
	"sync"
	"time"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/queue"
	"github.com/mongodb/amboy/reporting"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/recovery"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var globalEnv *envState

func init()                       { resetEnv() }
func GetEnvironment() Environment { return globalEnv }

func resetEnv() { globalEnv = &envState{name: "global", conf: &Configuration{}} }

type CloserFunc func(context.Context) error

// Environment objects provide access to shared configuration and
// state, in a way that you can isolate and test for in
type Environment interface {
	Configure(context.Context, *Configuration) error
	GetConf() (*Configuration, error)
	Context() (context.Context, context.CancelFunc)

	// GetQueue retrieves the application's shared queue, which is cache
	// for easy access from within units or inside of requests or command
	// line operations
	GetRemoteQueue() (amboy.Queue, error)
	SetRemoteQueue(amboy.Queue) error
	GetRemoteReporter() (reporting.Reporter, error)
	GetLocalQueue() (amboy.Queue, error)
	SetLocalQueue(amboy.Queue) error

	GetSession() (db.Session, error)
	GetClient() (*mongo.Client, error)
	GetDB() (*mongo.Database, error)

	RegisterCloser(string, CloserFunc)
	Close(context.Context) error
}

func GetSessionWithConfig(env Environment) (*Configuration, db.Session, error) {
	if env == nil {
		return nil, nil, errors.New("env is nil")
	}
	conf, err := env.GetConf()
	if err != nil {
		return nil, nil, errors.Wrap(err, "problem getting configuration")
	}

	session, err := env.GetSession()
	if err != nil {
		return nil, nil, errors.Wrap(err, "problem getting db session")
	}

	return conf, session, nil
}

type closerOp struct {
	name   string
	closer CloserFunc
}

type envState struct {
	name           string
	remoteQueue    amboy.Queue
	localQueue     amboy.Queue
	remoteReporter reporting.Reporter
	ctx            context.Context
	client         *mongo.Client
	conf           *Configuration
	closers        []closerOp
	mutex          sync.RWMutex
}

func (c *envState) Configure(ctx context.Context, conf *Configuration) error {
	var err error

	if err = conf.Validate(); err != nil {
		return errors.WithStack(err)
	}

	c.conf = conf
	c.client, err = mongo.NewClient(options.Client().ApplyURI(conf.MongoDBURI).SetConnectTimeout(conf.MongoDBDialTimeout).SetSocketTimeout(conf.SocketTimeout))
	if err != nil {
		return errors.Wrap(err, "problem constructing mongodb client")
	}

	connctx, cancel := context.WithTimeout(ctx, conf.MongoDBDialTimeout)
	defer cancel()
	if err := c.client.Connect(connctx); err != nil {
		return errors.Wrap(err, "problem connecting to database")
	}

	if !conf.DisableLocalQueue {
		c.localQueue = queue.NewLocalLimitedSize(conf.NumWorkers, 1024)
		grip.Infof("configured local queue with %d workers", conf.NumWorkers)

		c.RegisterCloser("local-queue", func(ctx context.Context) error {
			if !amboy.WaitCtxInterval(ctx, c.localQueue, 10*time.Millisecond) {
				grip.Critical(message.Fields{
					"message": "pending jobs failed to finish",
					"queue":   "system",
					"status":  c.localQueue.Stats(),
				})
				return errors.New("failed to stop with running jobs")
			}
			c.localQueue.Runner().Close()
			return nil
		})
	}

	if !conf.DisableRemoteQueue {
		opts := conf.GetQueueOptions()
		q := queue.NewRemoteUnordered(conf.NumWorkers)

		mongoDriver, err := queue.OpenNewMongoDriver(ctx, conf.QueueName, opts, c.client)
		if err != nil {
			return errors.Wrap(err, "problem opening db queue")
		}

		if err = q.SetDriver(mongoDriver); err != nil {
			return errors.Wrap(err, "problem configuring driver")
		}

		c.remoteQueue = q

		grip.Info(message.Fields{
			"message":  "configured a remote mongodb-backed queue",
			"db":       conf.DatabaseName,
			"prefix":   conf.QueueName,
			"priority": true})

		c.remoteReporter, err = reporting.MakeDBQueueState(ctx, conf.QueueName, opts, c.client)
		if err != nil {
			return errors.Wrap(err, "problem starting wrapper")
		}
	}

	var capturedCtxCancel context.CancelFunc
	c.ctx, capturedCtxCancel = context.WithCancel(ctx)
	c.RegisterCloser("env-captured-context-cancel", func(_ context.Context) error {
		capturedCtxCancel()
		return nil
	})

	return nil
}

func (c *envState) Context() (context.Context, context.CancelFunc) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return context.WithCancel(c.ctx)
}

func (c *envState) SetRemoteQueue(q amboy.Queue) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.remoteQueue != nil {
		return errors.New("remote queue exists, cannot overwrite")
	}

	if q == nil {
		return errors.New("cannot set remote queue to nil")
	}

	c.remoteQueue = q
	grip.Noticef("caching a '%T' remote queue in the '%s' service cache for use in tasks", q, c.name)
	return nil
}

func (c *envState) GetRemoteQueue() (amboy.Queue, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.remoteQueue == nil {
		return nil, errors.New("no remote queue defined in the services cache")
	}

	return c.remoteQueue, nil
}

func (c *envState) GetRemoteReporter() (reporting.Reporter, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.remoteReporter == nil {
		return nil, errors.New("no remote reporter")
	}

	return c.remoteReporter, nil
}

func (c *envState) SetLocalQueue(q amboy.Queue) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.localQueue != nil {
		return errors.New("local queue exists, cannot overwrite")
	}

	if q == nil {
		return errors.New("cannot set local queue to nil")
	}

	c.localQueue = q
	grip.Noticef("caching a '%T' local queue in the '%s' service cache for use in tasks", q, c.name)
	return nil
}

func (c *envState) GetLocalQueue() (amboy.Queue, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.localQueue == nil {
		return nil, errors.New("no local queue defined in the services cache")
	}

	return c.localQueue, nil
}

func (c *envState) GetSession() (db.Session, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.client == nil {
		return nil, errors.New("no valid session defined")
	}

	return db.WrapClient(c.ctx, c.client), nil
}

func (c *envState) GetClient() (*mongo.Client, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.client == nil {
		return nil, errors.New("client is not configured")
	}

	return c.client, nil
}

func (c *envState) GetDB() (*mongo.Database, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.client == nil {
		return nil, errors.New("client is not configured")
	}

	if c.conf.DatabaseName == "" {
		return nil, errors.New("database is not defined in the configuration")
	}

	return c.client.Database(c.conf.DatabaseName), nil
}

func (c *envState) GetConf() (*Configuration, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.conf == nil {
		return nil, errors.New("configuration is not set")
	}

	// copy the struct
	out := &Configuration{}
	*out = *c.conf

	return out, nil
}

func (c *envState) RegisterCloser(name string, op CloserFunc) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.closers = append(c.closers, closerOp{name: name, closer: op})
}

func (c *envState) Close(ctx context.Context) error {
	catcher := grip.NewExtendedCatcher()

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	deadline, _ := ctx.Deadline()
	wg := &sync.WaitGroup{}
	for _, closer := range c.closers {
		wg.Add(1)
		go func(name string, close CloserFunc) {
			defer wg.Done()
			defer recovery.LogStackTraceAndContinue("closing registered resources")

			grip.Info(message.Fields{
				"message":      "calling closer",
				"closer":       name,
				"timeout_secs": time.Until(deadline),
				"deadline":     deadline,
			})
			catcher.Add(close(ctx))
		}(closer.name, closer.closer)
	}

	wg.Wait()

	return catcher.Resolve()
}
