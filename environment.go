package sink

import (
	"sync"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/queue"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	mgo "gopkg.in/mgo.v2"
)

var globalEnv *envState

func init()                       { globalEnv = &envState{name: "global", conf: &Configuration{}} }
func GetEnvironment() Environment { return globalEnv }

type Loggers struct {
	System grip.Journaler
	Alerts grip.Journaler
	Events grip.Journaler
}

// Environment objects provide access to shared configuration and
// state, in a way that you can isolate and test for in
type Environment interface {
	Configure(*Configuration) error

	// GetQueue retrieves the application's shared queue, which is cache
	// for easy access from within units or inside of requests or command
	// line operations
	GetQueue() (amboy.Queue, error)

	GetConf() (*Configuration, error)
	// SetQueue configures the global application cache's shared queue.
	SetQueue(amboy.Queue) error
	GetSession() (db.Session, error)

	SetLoggers(Loggers) error

	// GetSystemLogger returns a grip.Journaler interface for use when
	// logging system events. When extending sink, you should generally log
	// messages using the default grip interface; hobwever, the system
	// event Sender and logger are available to log events to the database
	// or other services for more critical issues encoutered during offline
	// processing. In typical configurations these events are logged to
	// the database and exposed via a rest endpoint.
	GetSystemLogger() grip.Journaler

	// GetAlertLogger returns a grip.Journaler interface for use
	// when sending system alerts to administrators. Typically
	// this will be an interface to send email or slack messages
	// and should be used to alert on infrequent errors and events
	// that require manual intervention.
	GetAlertLogger() grip.Journaler

	// GetEventLogger returns a grip.Journaler interface for
	// sending data-events to an event logging
	// service. (e.g. splunk, sumologic, etc.)
	GetEventLogger() grip.Journaler
}

func GetSessionWithConfig(env Environment) (*Configuration, db.Session, error) {
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
	name    string
	queue   amboy.Queue
	session db.Session
	conf    *Configuration
	loggers Loggers

	mutex sync.RWMutex
}

func (c *envState) Configure(conf *Configuration) error {
	var err error

	c.conf = conf

	// create and cache a db session for use in tasks
	session, err := mgo.Dial(conf.MongoDBURI)
	if err != nil {
		return errors.Wrapf(err, "could not connect to db %s", conf.MongoDBURI)
	}

	c.session = db.WrapSession(session)

	if conf.UseLocalQueue {
		c.queue = queue.NewLocalLimitedSize(conf.NumWorkers, 1024)
		grip.Infof("configured local queue with %d workers", conf.NumWorkers)
	} else {
		q := queue.NewRemoteUnordered(conf.NumWorkers)
		opts := queue.MongoDBOptions{
			URI:      conf.MongoDBURI,
			DB:       conf.DatabaseName,
			Priority: true,
		}

		mongoDriver := queue.NewMongoDBDriver(QueueName, opts)
		if err = q.SetDriver(mongoDriver); err != nil {
			return errors.Wrap(err, "problem configuring driver")
		}

		c.queue = q

		grip.Info(message.Fields{
			"message":  "configured a remote mongodb-backed queue",
			"db":       conf.DatabaseName,
			"prefix":   QueueName,
			"priority": true})
	}

	return nil
}

func (c *envState) SetLoggers(l Loggers) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.loggers = l

	return nil
}

func (c *envState) SetQueue(q amboy.Queue) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.queue != nil {
		return errors.New("queue exists, cannot overwrite")
	}

	if q == nil {
		return errors.New("cannot set queue to nil")
	}

	c.queue = q
	grip.Noticef("caching a '%T' queue in the '%s' service cache for use in tasks", q, c.name)
	return nil
}

func (c *envState) GetQueue() (amboy.Queue, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.queue == nil {
		return nil, errors.New("no queue defined in the services cache")
	}

	return c.queue, nil
}

func (c *envState) GetSession() (db.Session, error) {
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

func (c *envState) GetSystemLogger() grip.Journaler {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.loggers.System
}

func (c *envState) GetAlertLogger() grip.Journaler {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.loggers.Alerts
}

func (c *envState) GetEventLogger() grip.Journaler {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.loggers.Events
}
