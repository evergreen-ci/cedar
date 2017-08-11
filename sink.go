/*
Package sink holds a a number of application level constants and
shared resources for the sink application.

Services Cache

The sink package maintains a public interface to a shared cache of
interfaces and services for use in building tools within sink. The
sink package has no dependencies to any sub-packages, and all methods
in the public interface are thread safe.

In practice these values are set in the operations package. See
sink/operations/setup.go for details.
*/
package sink

import (
	"sync"

	"github.com/evergreen-ci/sink/cost"
	"github.com/mongodb/amboy"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/logging"
	"github.com/mongodb/grip/send"
	"github.com/pkg/errors"
	mgo "gopkg.in/mgo.v2"
)

// Sink defines a number of application level constants used
// throughout the application.
const (
	QueueName = "queue"
)

// BuildRevision stores the commit in the git repository at build time
// and is specified with -ldflags at build time
var BuildRevision = ""

var servicesCache *appServicesCache

func init() {
	servicesCache = &appServicesCache{
		name: "global",
	}
}

// SetQueue configures the global application cache's shared queue.
func SetQueue(q amboy.Queue) error { return servicesCache.setQueue(q) }

// GetQueue retrieves the application's shared queue, which is cache
// for easy access from within units or inside of requests or command
// line operations
func GetQueue() (amboy.Queue, error) { return servicesCache.getQueue() }

// SetMgoSession set's the application's mgo session for later use in
// conjunction with GetMgoSession by models and work units.
func SetMgoSession(s *mgo.Session) error { return servicesCache.setMgoSession(s) }

// GetMgoSession returns a copy of the session, a database, and an
// error for use in a local operation. Callers should be sure to close
// the session when their operation is complete; typically by using a
// "defer session.Closed()" operation.
func GetMgoSession() (*mgo.Session, *mgo.Database, error) { return servicesCache.getMgoSession() }

// SetConf register's the application configuration, replacing an
// existing configuration as needed.
func SetConf(conf *Configuration) { servicesCache.setConf(conf) }

// GetConf returns a copy of the global configuration object. Even
// though the method returns a pointer, the underlying data is copied.
func GetConf() *Configuration { return servicesCache.getConf() }

// GetSystemSender returns a grip/send.Sender interface for use when
// logging system events. When extending sink, you should generally log
// messages using the default grip interface; however, the system
// event Sender and logger are available to log events to the database
// or other services for more critical issues encoutered during offline
// processing. In typical configurations these events are logged to
// the database and exposed via a rest endpoint.
func GetSystemSender() send.Sender { return servicesCache.sysSender }

// SetSystemSender configures the system logger.
func SetSystemSender(s send.Sender) error { return servicesCache.setSeystemEventLog(s) }

// GetLogger returns a compatible grip.Jounaler interface for use in
// logging offline issues to the database.
func GetLogger() grip.Journaler { return servicesCache.getLogger() }

// GetSpendConfig returns the current configuration settings for the spend
// command line tool.
func GetSpendConfig() *cost.Config { return servicesCache.getSpendConfig() }

// SetSpendConfig updates the configuration settings for the spend command
// line tool using the given yaml file.
func SetSpendConfig(file string) error { return servicesCache.setSpendConfig(file) }

////////////////////////////////////////////////////////////////////////
//
// internal implementation of the cache

// see the documentation for the corresponding global methods for

type appServicesCache struct {
	name        string
	queue       amboy.Queue
	session     *mgo.Session
	conf        *Configuration
	sysSender   send.Sender
	logger      grip.Journaler
	spendConfig *cost.Config

	mutex sync.RWMutex
}

func (c *appServicesCache) setQueue(q amboy.Queue) error {
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

func (c *appServicesCache) getQueue() (amboy.Queue, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.queue == nil {
		return nil, errors.New("no queue defined in the services cache")
	}

	return c.queue, nil
}

func (c *appServicesCache) setMgoSession(s *mgo.Session) error {
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

func (c *appServicesCache) getMgoSession() (*mgo.Session, *mgo.Database, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.session == nil {
		return nil, nil, errors.New("no valid session defined")
	}

	if c.conf.DatabaseName == "" {
		return nil, nil, errors.New("no database defined")
	}

	s := c.session.Clone()
	return s, s.DB(c.conf.DatabaseName), nil
}

func (c *appServicesCache) setConf(conf *Configuration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.conf = conf
}

func (c *appServicesCache) getConf() *Configuration {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// copy the struct
	out := Configuration{}
	out = *c.conf

	return &out
}

func (c *appServicesCache) setSeystemEventLog(s send.Sender) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.sysSender = s
	c.logger = logging.NewGrip("sink")
	return errors.WithStack(c.logger.SetSender(s))
}

func (c *appServicesCache) getSystemEventLog() send.Sender { // nolint: megacheck
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.sysSender
}

func (c *appServicesCache) getLogger() grip.Journaler {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.logger
}

func (c *appServicesCache) getSpendConfig() *cost.Config {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	if c.spendConfig == nil {
		c.spendConfig = &cost.Config{}
	}
	return c.spendConfig
}

func (c *appServicesCache) setSpendConfig(file string) error {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	newConfig, err := cost.YAMLToConfig(file)
	if err != nil {
		return err
	}
	configFile := c.getSpendConfig()
	if newConfig.Opts.Directory != "" {
		configFile.Opts.Directory = newConfig.Opts.Directory
	}
	if newConfig.Opts.Duration != "" {
		configFile.Opts.Duration = newConfig.Opts.Duration
	}
	if newConfig.EvergreenInfo != nil {
		configFile.EvergreenInfo = newConfig.EvergreenInfo
	}
	if newConfig.Pricing != nil {
		configFile.Pricing = newConfig.Pricing
	}
	if newConfig.S3Info != nil {
		configFile.S3Info = newConfig.S3Info
	}
	if len(newConfig.Accounts) != 0 {
		configFile.Accounts = newConfig.Accounts
	}
	if configFile.Providers != nil {
		configFile.UpdateSpendProviders(newConfig.Providers)
	} else {
		configFile.Providers = newConfig.Providers
	}
	return nil
}
