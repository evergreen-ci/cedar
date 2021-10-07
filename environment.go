package cedar

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/management"
	"github.com/mongodb/amboy/pool"
	"github.com/mongodb/amboy/queue"
	"github.com/mongodb/anser/apm"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/recovery"
	"github.com/mongodb/jasper"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	envlock   *sync.RWMutex
	globalEnv Environment
)

func init() {
	envlock = &sync.RWMutex{}
	SetEnvironment(&envState{name: "cedar-init", conf: &Configuration{}})
}

func SetEnvironment(env Environment) {
	envlock.Lock()
	defer envlock.Unlock()
	globalEnv = env
}

func GetEnvironment() Environment {
	envlock.RLock()
	defer envlock.RUnlock()
	return globalEnv
}

func NewEnvironment(ctx context.Context, name string, conf *Configuration) (Environment, error) {
	env := &envState{serverCertVersion: -1}

	var err error

	if err = conf.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	env.conf = conf
	if env.client == nil {
		opts := options.Client().ApplyURI(conf.MongoDBURI).
			SetConnectTimeout(conf.MongoDBDialTimeout).
			SetSocketTimeout(conf.SocketTimeout).
			SetServerSelectionTimeout(conf.SocketTimeout).
			SetMonitor(apm.NewLoggingMonitor(ctx, time.Minute, apm.NewBasicMonitor(&apm.MonitorConfig{AllTags: true})).DriverAPM())
		if conf.HasAuth() {
			credential := options.Credential{
				Username: conf.DBUser,
				Password: conf.DBPwd,
			}
			opts.SetAuth(credential)
		}
		env.client, err = mongo.NewClient(opts)
		if err != nil {
			return nil, errors.Wrap(err, "problem constructing mongodb client")
		}
		if err = env.client.Ping(ctx, nil); err != nil {
			connctx, cancel := context.WithTimeout(ctx, conf.MongoDBDialTimeout)
			defer cancel()
			if err = env.client.Connect(connctx); err != nil {
				return nil, errors.Wrap(err, "problem connecting to database")
			}
		}
	}
	if !conf.DisableDBValueCaching {
		env.dbValueCache = map[string]interface{}{}
	}

	if !conf.DisableLocalQueue {
		env.localQueue = queue.NewLocalLimitedSize(conf.NumWorkers, 1024)
		grip.Infof("configured local queue with %d workers", conf.NumWorkers)

		env.RegisterCloser("local-queue", func(ctx context.Context) error {
			if !amboy.WaitInterval(ctx, env.localQueue, 10*time.Millisecond) {
				grip.Critical(message.Fields{
					"message": "pending jobs failed to finish",
					"queue":   "system",
					"status":  env.localQueue.Stats(ctx),
				})
				return errors.New("failed to stop with running jobs")
			}
			env.localQueue.Runner().Close(ctx)
			return nil
		})

		if err = env.localQueue.Start(ctx); err != nil {
			return nil, errors.Wrap(err, "problem starting remote queue")
		}
	}

	if !conf.DisableRemoteQueue {
		opts := conf.GetQueueOptions()

		args := queue.MongoDBQueueCreationOptions{
			Size:    conf.NumWorkers,
			Name:    conf.QueueName,
			Ordered: false,
			Client:  env.client,
			MDB:     opts,
			Retryable: queue.RetryableQueueOptions{
				RetryHandler: amboy.RetryHandlerOptions{
					NumWorkers:       2,
					MaxCapacity:      4096,
					MaxRetryAttempts: 10,
					MaxRetryTime:     15 * time.Minute,
					RetryBackoff:     10 * time.Second,
				},
				StaleRetryingMonitorInterval: time.Minute,
			},
		}

		var rq amboy.Queue
		rq, err = queue.NewMongoDBQueue(ctx, args)
		if err != nil {
			return nil, errors.Wrap(err, "problem setting main queue backend")
		}

		if err = rq.SetRunner(pool.NewAbortablePool(conf.NumWorkers, rq)); err != nil {
			return nil, errors.Wrap(err, "problem configuring worker pool for main remote queue")
		}
		env.remoteQueue = rq
		env.RegisterCloser("application-queue", func(ctx context.Context) error {
			env.remoteQueue.Runner().Close(ctx)
			return nil
		})

		grip.Info(message.Fields{
			"message":  "configured a remote mongodb-backed queue",
			"db":       conf.DatabaseName,
			"prefix":   conf.QueueName,
			"priority": true})

		if err = env.remoteQueue.Start(ctx); err != nil {
			return nil, errors.Wrap(err, "problem starting remote queue")
		}
		managementOpts := management.DBQueueManagerOptions{
			Name:    conf.QueueName,
			Options: opts,
		}
		env.remoteManager, err = management.MakeDBQueueManager(ctx, managementOpts, env.client)
		if err != nil {
			return nil, errors.Wrap(err, "problem starting remote reporter")
		}
	}

	if !conf.DisableRemoteQueueGroup {
		opts := conf.GetQueueGroupOptions()
		args := queue.MongoDBQueueGroupOptions{
			Prefix:                    conf.QueueName,
			DefaultWorkers:            conf.NumWorkers,
			Ordered:                   false,
			BackgroundCreateFrequency: 10 * time.Minute,
			PruneFrequency:            10 * time.Minute,
			TTL:                       time.Minute,
			Retryable: queue.RetryableQueueOptions{
				RetryHandler: amboy.RetryHandlerOptions{
					NumWorkers:       2,
					MaxCapacity:      4096,
					MaxRetryAttempts: 10,
					MaxRetryTime:     15 * time.Minute,
					RetryBackoff:     10 * time.Second,
				},
				StaleRetryingMonitorInterval: time.Minute,
			},
		}

		env.remoteQueueGroup, err = queue.NewMongoDBSingleQueueGroup(ctx, args, env.client, opts)
		if err != nil {
			return nil, errors.Wrap(err, "problem starting remote queue group")
		}

		env.RegisterCloser("remote-queue-group", func(ctx context.Context) error {
			return errors.Wrap(env.remoteQueueGroup.Close(ctx), "problem waiting for remote queue group to close")
		})
	}

	jpm, err := jasper.NewSynchronizedManager(true)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	env.jpm = jpm

	env.RegisterCloser("jasper-manager", func(ctx context.Context) error {
		return errors.WithStack(jpm.Close(ctx))
	})

	var capturedCtxCancel context.CancelFunc
	env.ctx, capturedCtxCancel = context.WithCancel(ctx)
	env.RegisterCloser("env-captured-context-cancel", func(_ context.Context) error {
		capturedCtxCancel()
		return nil
	})

	return env, nil
}

// Environment objects provide access to shared configuration and
// state, in a way that you can isolate and test for in
type Environment interface {
	GetConf() *Configuration
	Context() (context.Context, context.CancelFunc)

	// GetQueue retrieves the application's shared queue, which is cached
	// for easy access from within units or inside of requests or command
	// line operations.
	GetRemoteQueue() amboy.Queue
	SetRemoteQueue(amboy.Queue) error
	GetRemoteManager() management.Manager
	GetLocalQueue() amboy.Queue
	SetLocalQueue(amboy.Queue) error

	GetRemoteQueueGroup() amboy.QueueGroup

	GetSession() db.Session
	GetClient() *mongo.Client
	GetDB() *mongo.Database
	RegisterDBValueCacher(string, interface{}, chan interface{}) bool
	GetCachedDBValue(string) (interface{}, bool)
	Jasper() jasper.Manager

	GetServerCertVersion() int
	SetServerCertVersion(i int)

	RegisterCloser(string, CloserFunc)
	Close(context.Context) error
}

func GetSessionWithConfig(env Environment) (*Configuration, db.Session, error) {
	if env == nil {
		return nil, nil, errors.New("env is nil")
	}

	conf := env.GetConf()
	if conf == nil {
		return nil, nil, errors.New("conf is nil")
	}

	session := env.GetSession()
	if session == nil {
		return nil, nil, errors.New("session is nil")
	}

	return conf, session, nil
}

type CloserFunc func(context.Context) error

type closerOp struct {
	name   string
	closer CloserFunc
}

type envState struct {
	name              string
	remoteQueue       amboy.Queue
	localQueue        amboy.Queue
	remoteQueueGroup  amboy.QueueGroup
	remoteManager     management.Manager
	ctx               context.Context
	client            *mongo.Client
	dbValueCache      map[string]interface{}
	conf              *Configuration
	jpm               jasper.Manager
	serverCertVersion int
	closers           []closerOp
	mutex             sync.RWMutex
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

func (c *envState) GetRemoteQueue() amboy.Queue {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.remoteQueue
}

func (c *envState) GetRemoteQueueGroup() amboy.QueueGroup {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.remoteQueueGroup
}

func (c *envState) GetRemoteManager() management.Manager {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.remoteManager
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

func (c *envState) GetLocalQueue() amboy.Queue {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.localQueue
}

func (c *envState) GetSession() db.Session {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.client == nil {
		return nil
	}

	return db.WrapClient(c.ctx, c.client).Clone()
}

func (c *envState) GetClient() *mongo.Client {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.client
}

func (c *envState) GetDB() *mongo.Database {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.client == nil {
		return nil
	}

	if c.conf.DatabaseName == "" {
		return nil
	}

	return c.client.Database(c.conf.DatabaseName)
}

func (c *envState) RegisterDBValueCacher(name string, val interface{}, updateChan chan interface{}) bool {
	c.mutex.Lock()
	if c.dbValueCache == nil {
		c.mutex.Unlock()
		return false
	}
	if _, ok := c.dbValueCache[name]; ok {
		c.mutex.Unlock()
		return false
	}
	c.dbValueCache[name] = val
	c.mutex.Unlock()

	go func() {
		defer recovery.LogStackTraceAndContinue(fmt.Sprintf("env database value cache updater for '%s'", name))

		select {
		case newVal := <-updateChan:
			c.mutex.Lock()
			switch newVal.(type) {
			case error:
				// If there is an error, we should clear this
				// value from the cache and exit the goroutine
				// so the caller knows to register another
				// value cacher.
				delete(c.dbValueCache, name)
				c.mutex.Unlock()
				return
			default:
				c.dbValueCache[name] = newVal
			}
			c.mutex.Unlock()
		case <-c.ctx.Done():
			return
		}
	}()

	return true
}

func (c *envState) GetCachedDBValue(name string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.dbValueCache == nil {
		return nil, false
	}

	val, ok := c.dbValueCache[name]
	return val, ok
}

func (c *envState) GetConf() *Configuration {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.conf == nil {
		return nil
	}

	// copy the struct
	out := &Configuration{}
	*out = *c.conf

	return out
}

func (c *envState) GetServerCertVersion() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.serverCertVersion
}

func (c *envState) SetServerCertVersion(i int) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if i > c.serverCertVersion {
		c.serverCertVersion = i
	}
}

func (c *envState) RegisterCloser(name string, op CloserFunc) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.closers = append(c.closers, closerOp{name: name, closer: op})
}

func (c *envState) Jasper() jasper.Manager {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.jpm
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
