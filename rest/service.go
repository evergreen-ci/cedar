package rest

import (
	"runtime"

	mgo "gopkg.in/mgo.v2"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/queue"
	"github.com/mongodb/amboy/queue/driver"
	"github.com/pkg/errors"
	"github.com/tychoish/gimlet"
	"github.com/tychoish/grip"
	"github.com/tychoish/sink"
	"golang.org/x/net/context"
)

const (
	dbName    = "sink"
	queueName = "queue"
)

type Service struct {
	Workers    int
	MongoDBURI string
	Port       int

	// internal settings
	queue amboy.Queue
	app   *gimlet.APIApp
}

func (s *Service) Validate() error {
	if s.Workers <= 0 {
		s.Workers = runtime.NumCPU()
	}

	// TODO make it possible to have a local queue but still pass a MongoDBURI
	// TODO move default setting into a seperate function

	if s.MongoDBURI == "" {
		s.queue = queue.NewLocalUnordered(s.Workers)
		grip.Infof("configured a local queue with %d workers", s.Workers)
	} else {
		remoteQueue := queue.NewRemoteUnordered(runtime.NumCPU())
		opts := driver.MongoDBOptions{
			URI:      s.MongoDBURI,
			DB:       dbName,
			Priority: true,
		}
		if err := sink.SetDriverOpts(queueName, opts); err != nil {
			return errors.Wrap(err, "problem caching driver options")
		}

		mongoDriver := driver.NewMongoDB(queueName, opts)
		if err := remoteQueue.SetDriver(mongoDriver); err != nil {
			return errors.Wrap(err, "problem configuring driver")
		}
		s.queue = remoteQueue
		grip.Infof("configured a remote mongodb-backed queue "+
			"[db=%s, prefix=%s, priority=%t]", dbName, queueName, true)

		// create and cache a db session for use in
		// tasks. this should probably move elsewhere.
		session, err := mgo.Dial(s.MongoDBURI)
		if err != nil {
			return errors.Wrapf(err, "could not connect to db %s", s.MongoDBURI)
		}

		if err := sink.SetMgoSession(session); err != nil {
			return errors.Wrap(err, "problem caching DB session")
		}
	}

	if err := sink.SetQueue(s.queue); err != nil {
		return errors.Wrap(err, "problem caching amboy queue")
	}

	if s.Port == 0 {
		s.Port = 3000
	}

	s.app = gimlet.NewApp()
	s.app.SetDefaultVersion(1)
	if err := s.app.SetPort(s.Port); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (s *Service) Start(ctx context.Context) error {
	grip.NoticeWhenf(s.MongoDBURI == "", "sink service on port %d, with local queue", s.Port)
	grip.NoticeWhenf(s.MongoDBURI != "", "sink service on port %d, with db-backed queue", s.Port)

	if s.queue == nil || s.app == nil {
		return errors.New("application is not valid")
	}

	s.addRoutes()

	if err := s.queue.Start(ctx); err != nil {
		return errors.Wrap(err, "problem starting queue")
	}

	if err := s.app.Resolve(); err != nil {
		return errors.Wrap(err, "problem resolving routes")
	}

	if err := s.app.Run(); err != nil {
		return errors.Wrap(err, "problem running service")
	}

	grip.Noticef("completed sink service; shutting down")

	return nil
}

func (s *Service) addRoutes() {
	s.app.AddRoute("/status").Version(1).Get().Handler(s.statusHandler)
	s.app.AddRoute("/simple_log/{id}").Version(1).Post().Handler(s.simpleLogInjestion)
}
