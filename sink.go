package sink

import (
	"sync"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/queue/driver"
	"github.com/pkg/errors"
	mgo "gopkg.in/mgo.v2"
)

// Should be specified with -ldflags at build time
var BuildRevision = ""

var servicesCache *appServicesCache

func init() {
	servicesCache = &appServicesCache{}
}

type appServicesCache struct {
	queue      amboy.Queue
	driverOpts driver.MongoDBOptions
	session    *mgo.Session

	mutex sync.RWMutex
}

func SetQueue(q amboy.Queue) error {
	servicesCache.mutex.Lock()
	defer servicesCache.mutex.Unlock()

	if servicesCache.queue != nil {
		return errors.New("queue exists, cannot overwrite")
	}

	if q == nil {
		return errors.New("cannot set queue to nil")
	}

	servicesCache.queue = q
	return nil
}

func SetDriverOpts(opts driver.MongoDBOptions) error {
	servicesCache.mutex.Lock()
	defer servicesCache.mutex.Unlock()

	if opts.URI == "" || opts.DB == "" {
		return errors.Errorf("driver options %+v is not valid", opts)
	}

	servicesCache.driverOpts = opts
	return nil
}

func GetQueue() (amboy.Queue, error) {
	servicesCache.mutex.RLock()
	defer servicesCache.mutex.RUnlock()

	if servicesCache.queue == nil {
		return nil, errors.New("no queue defined in the services cache")
	}

	return servicesCache.queue, nil
}

func SetMgoSession(s *mgo.Session) error {
	servicesCache.mutex.Lock()
	defer servicesCache.mutex.Unlock()

	if servicesCache.session != nil {
		return errors.New("cannot set a session since it already exists")
	}

	if s == nil {
		return errors.New("cannot use a nil session")
	}

	servicesCache.session = s
	return nil
}

func GetMgoSession() (*mgo.Session, error) {
	servicesCache.mutex.RLock()
	defer servicesCache.mutex.RUnlock()

	if servicesCache.session == nil {
		return nil, errors.New("no valid session defined")
	}

	s := servicesCache.session.Clone()
	return s, nil
}
