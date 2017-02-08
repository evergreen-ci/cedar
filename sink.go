package sink

import (
	"sync"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/queue/driver"
	"github.com/pkg/errors"
	"github.com/tychoish/grip"
	mgo "gopkg.in/mgo.v2"
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

func SetMgoSession(s *mgo.Session) error   { return servicesCache.setMgoSession(s) }
func GetMgoSession() (*mgo.Session, error) { return servicesCache.getMgoSession() }

////////////////////////////////////////////////////////////////////////
//
// internal implementation of the cache

type appServicesCache struct {
	name            string
	queue           amboy.Queue
	driverQueueName string
	driverOpts      driver.MongoDBOptions
	session         *mgo.Session

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

func (c *appServicesCache) getMgoSession() (*mgo.Session, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.session == nil {
		return nil, errors.New("no valid session defined")
	}

	s := c.session.Clone()
	return s, nil
}
