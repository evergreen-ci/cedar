package sink

import (
	"sync"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/queue/driver"
	"github.com/pkg/errors"
	"github.com/tychoish/grip"
	mgo "gopkg.in/mgo.v2"
)

const (
	QueueName = "queue"
)

// Should be specified with -ldflags at build time
var BuildRevision = ""

var servicesCache *appServicesCache

func init() {
	servicesCache = &appServicesCache{
		name: "global",
	}
}

func SetQueue(q amboy.Queue) error   { return servicesCache.setQueue(q) }
func GetQueue() (amboy.Queue, error) { return servicesCache.getQueue() }

func SetDriverOpts(name string, opts driver.MongoDBOptions) error {
	return servicesCache.setDriverOpts(name, opts)
}

func SetMgoSession(s *mgo.Session) error                  { return servicesCache.setMgoSession(s) }
func GetMgoSession() (*mgo.Session, *mgo.Database, error) { return servicesCache.getMgoSession() }

func SetConf(conf *SinkConfiguration) { servicesCache.setConf(conf) }
func GetConf() *SinkConfiguration     { return servicesCache.getConf() }

////////////////////////////////////////////////////////////////////////
//
// internal implementation of the cache

type appServicesCache struct {
	name            string
	queue           amboy.Queue
	driverQueueName string
	driverOpts      driver.MongoDBOptions
	session         *mgo.Session
	conf            *SinkConfiguration

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

func (c *appServicesCache) setDriverOpts(name string, opts driver.MongoDBOptions) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if opts.URI == "" || opts.DB == "" || name == "" {
		return errors.Errorf("driver options %+v is not valid", opts)
	}

	c.driverOpts = opts
	return nil
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

func (c *appServicesCache) setConf(conf *SinkConfiguration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.conf = conf
}

func (c *appServicesCache) getConf() *SinkConfiguration {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// copy the struct
	out := SinkConfiguration{}
	out = *c.conf

	return &out
}
