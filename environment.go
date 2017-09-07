package sink

import (
	"sync"

	"github.com/mongodb/amboy"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/logging"
	"github.com/mongodb/grip/send"
	"github.com/pkg/errors"
	"github.com/tychoish/anser/db"
)

var globalEnv *envState

func init()                       { globalEnv = &envState{name: "global"} }
func GetEnvironment() Environment { return globalEnv }

// Environment objects provide access to shared configuration and
// state, in a way that you can isolate and test for in
type Environment interface {
	// GetQueue retrieves the application's shared queue, which is cache
	// for easy access from within units or inside of requests or command
	// line operations
	GetQueue() (amboy.Queue, error)

	GetConf() (*Configuration, error)
	SetConf(*Configuration) error
	// SetQueue configures the global application cache's shared queue.
	SetQueue(amboy.Queue) error
	GetSession() (db.Session, error)
	SetSession(db.Session) error

	// GetSender returns a grip/send.Sender interface for use when
	// logging system events. When extending sink, you should generally log
	// messages using the default grip interface; however, the system
	// event Sender and logger are available to log events to the database
	// or other services for more critical issues encoutered during offline
	// processing. In typical configurations these events are logged to
	// the database and exposed via a rest endpoint.
	GetSender() send.Sender
	SetSender(send.Sender) error
	GetLogger() grip.Journaler
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
	name      string
	queue     amboy.Queue
	session   db.Session
	conf      *Configuration
	sysSender send.Sender
	logger    grip.Journaler

	mutex sync.RWMutex
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

func (c *envState) SetSession(s db.Session) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.session != nil {
		return errors.New("cannot set a session since it already exists")
	}

	if s == nil {
		return errors.New("cannot use a nil session")
	}

	grip.Notice("caching a mongodb session in the services cache")

	c.session = s
	return nil
}

func (c *envState) GetSession() (db.Session, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.session == nil {
		return nil, errors.New("no valid session defined")
	}

	return c.session.Clone(), nil
}

func (c *envState) SetConf(conf *Configuration) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if conf == nil {
		return errors.New("cannot set configuration to nil")
	}

	c.conf = conf
	return nil
}

func (c *envState) GetConf() (*Configuration, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.conf == nil {
		return nil, errors.New("configuration is not set")
	}

	// copy the struct
	out := Configuration{}
	out = *c.conf

	return &out, nil
}

func (c *envState) SetSender(s send.Sender) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.sysSender = s
	c.logger = logging.NewGrip("sink")
	return errors.WithStack(c.logger.SetSender(s))
}

func (c *envState) GetSender() send.Sender { // nolint: megacheck
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.sysSender
}

func (c *envState) GetLogger() grip.Journaler {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.logger
}
