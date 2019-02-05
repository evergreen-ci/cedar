package cedar

import (
	"context"
	"sync"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/queue"
	"github.com/mongodb/amboy/reporting"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	mgo "gopkg.in/mgo.v2"
)

var globalEnv *envState

func init()                       { resetEnv() }
func GetEnvironment() Environment { return globalEnv }

func resetEnv() { globalEnv = &envState{name: "global", conf: &Configuration{}} }

// Environment objects provide access to shared configuration and
// state, in a way that you can isolate and test for in
type Environment interface {
	Configure(*Configuration) error

	// GetQueue retrieves the application's shared queue, which is cache
	// for easy access from within units or inside of requests or command
	// line operations
	GetRemoteQueue() (amboy.Queue, error)
	SetRemoteQueue(amboy.Queue) error
	GetRemoteReporter() (reporting.Reporter, error)

	GetLocalQueue() (amboy.Queue, error)
	SetLocalQueue(amboy.Queue) error

	GetConf() (*Configuration, error)
	// SetQueue configures the global application cache's shared queue.
	GetSession() (*mgo.Session, error)
}

func GetSessionWithConfig(env Environment) (*Configuration, *mgo.Session, error) {
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

type envState struct {
	name           string
	remoteQueue    amboy.Queue
	remoteReporter remporting.Reporter
	localQueue     amboy.Queue
	session        *mgo.Session
	conf           *Configuration
	mutex          sync.RWMutex
}

func (c *envState) Configure(conf *Configuration) error {
	var err error

	if err = conf.Validate(); err != nil {
		return errors.WithStack(err)
	}

	c.conf = conf

	// create and cache a db session for use in tasks
	c.session, err = mgo.DialWithTimeout(conf.MongoDBURI, conf.MongoDBDialTimeout)
	if err != nil {
		return errors.Wrapf(err, "could not connect to db %s", conf.MongoDBURI)
	}
	c.session.SetSocketTimeout(conf.SocketTimeout)

	if !conf.DisableLocalQueue {
		c.LocalQueue = queue.NewLocalLimitedSize(conf.NumWorkers, 1024)
		grip.Infof("configured local queue with %d workers", conf.NumWorkers)
	}

	if !conf.DisableRemoteQueue {
		q := queue.NewRemoteUnordered(conf.NumWorkers)
		mongoDriver, err := queue.OpenNewMgoDriver(context.TODO(), QueueName, conf.GetQueueOptions(), c.session)
		if err != nil {
			return errors.Wrap(err, "problem opening db queue")
		}

		if err = q.SetDriver(mongoDriver); err != nil {
			return errors.Wrap(err, "problem configuring driver")
		}

		c.remoteQueue = q

		grip.Info(message.Fields{
			"message":  "configured a remote mongodb-backed queue",
			"db":       conf.QueueDatabaseName,
			"prefix":   QueueName,
			"priority": true})

		c.remoteReporter, err = reporting.MakeDBQueueState(QueueName, opts, c.session)
		if err != nil {
			return errors.Wrap(err, "problem starting wrapper")
		}

	}

	return nil
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
func (c *envState) GetRemoteQueue() (amboy.Queue, error) {
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

	if c.queue != nil {
		return errors.New("local queue exists, cannot overwrite")
	}

	if q == nil {
		return errors.New("cannot set local queue to nil")
	}

	c.queue = q
	grip.Noticef("caching a '%T' local queue in the '%s' service cache for use in tasks", q, c.name)
	return nil
}

func (c *envState) GetLocalQueue() (amboy.Queue, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.queue == nil {
		return nil, errors.New("no local queue defined in the services cache")
	}

	return c.queue, nil
}

func (c *envState) GetSession() (*mgo.Session, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.session == nil {
		return nil, errors.New("no valid session defined")
	}

	return c.session.Clone(), nil
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
