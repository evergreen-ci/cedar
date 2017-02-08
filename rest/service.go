package rest

import (
	"runtime"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/queue"
	"github.com/mongodb/amboy/queue/driver"
	"github.com/pkg/errors"
	"github.com/tychoish/gimlet"
	"github.com/tychoish/grip"
	"golang.org/x/net/context"
)

const (
	dBName    = "sink"
	queueName = "queue"
)

type Service struct {
	Workers    int
	MongoDBURI string
	Port

	// internal settings
	queue amboy.Queue
	app   *gimlet.APIApp
}

func (s *Service) Validate() error {
	if s.Workers <= 0 {
		s.Workers = runtime.NumCPU()
	}

	if s.MongoDBURI == "" {
		s.queue = queue.NewLocalUnordered(s.Workers)
		grip.Infof("configured a local queue with %d workers", s.Workers)
	} else {
		remoteQueue := queue.RemoteUnordered(runtime.NumCPU())
		mongoDriver := driver.NewMongoDB(dbPrefix, driver.MongoDBOptions{
			URI:      s.MongoDBURI,
			DB:       dBName,
			Priority: true,
		})

		if err := remoteQueue.SetDriver(mongoDriver); err != nil {
			return errors.Wrap(err, "problem configuring driver")
		}
		grip.Infof("configured a remote mongodb-backed queue "+
			"[db=%s, prefix=%s, priority=%t]", dbName, queueName, true)
	}

	if s.Port == 0 {
		s.Port == 3000
	}

	s.app = gimlet.NewApp()
	s.app.SetDefaultVersion(1)
	if err := s.app.SetPort(s.Port); err != nil {
		return errors.WithStack(err)
	}
}

func (s *Service) Start(ctx context.Context) error {
	grip.NoticeWhenf(s.MongoDBURI == "", "sink service on port %s, with local queue", s.Port)
	grip.NoticeWhenf(s.MongoDBURI != "", "sink service on port %s, with db-backed queue", s.Port)

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
}

func (s *Service) addRoutes() {
	app := s.app.App()

	app.AddRoute("/status").Version(1).Get().Handler(s.statusHandler)
}
