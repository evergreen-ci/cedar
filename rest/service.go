package rest

import (
	"context"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/amboy"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

type Service struct {
	Port        int
	Prefix      string
	Environment cedar.Environment
	RPCServers  []string

	// internal settings
	queue amboy.Queue
	app   *gimlet.APIApp
	sc    data.Connector
}

func (s *Service) Validate() error {
	var err error

	if s.Environment == nil {
		return errors.New("must specify an environment")
	}

	if s.queue == nil {
		s.queue, err = s.Environment.GetQueue()
		if err != nil {
			return errors.Wrap(err, "problem getting queue")
		}
		if s.queue == nil {
			return errors.New("no queue defined")
		}
	}

	if s.app == nil {
		s.app = gimlet.NewApp()
	}

	if s.sc == nil {
		s.sc = data.CreateNewDBConnector(s.Environment)
	}

	if s.Port == 0 {
		s.Port = 3000
	}

	if err := s.app.SetPort(s.Port); err != nil {
		return errors.WithStack(err)
	}

	if s.Prefix != "" {
		s.app.SetPrefix(s.Prefix)
	}

	if s.RPCServers == nil {
		addr, err := util.GetPublicIP()
		if err != nil {
			return errors.WithStack(err)
		}

		s.RPCServers = []string{addr}
	}

	grip.Info(message.Fields{
		"message": "detected local rpc services",
		"service": s.RPCServers,
	})
	return nil
}

func (s *Service) Start(ctx context.Context) error {
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

	return s.app.Run(ctx)
}

func (s *Service) addRoutes() {
	s.app.AddRoute("/admin/status").Version(1).Get().Handler(s.statusHandler)
	s.app.AddRoute("/admin/status/event/{id}").Version(1).Get().Handler(s.getSystemEvent)
	s.app.AddRoute("/admin/status/event/{id}/acknowledge").Version(1).Get().Handler(s.acknowledgeSystemEvent)
	s.app.AddRoute("/admin/status/events/{level}").Version(1).Get().Handler(s.getSystemEvents)
	s.app.AddRoute("/admin/service/flag/{flagName}/enabled").Version(1).Post().Handler(s.setServiceFlagEnabled)
	s.app.AddRoute("/admin/service/flag/{flagName}/disabled").Version(1).Post().Handler(s.setServiceFlagDisabled)

	s.app.AddRoute("/simple_log/{id}").Version(1).Post().Handler(s.simpleLogInjestion)
	s.app.AddRoute("/simple_log/{id}").Version(1).Get().Handler(s.simpleLogRetrieval)
	s.app.AddRoute("/simple_log/{id}/text").Version(1).Get().Handler(s.simpleLogGetText)
	s.app.AddRoute("/system_info").Version(1).Post().Handler(s.recieveSystemInfo)
	s.app.AddRoute("/system_info/host/{host}").Version(1).Post().Handler(s.fetchSystemInfo)

	s.app.AddRoute("/depgraph/{id}").Version(1).Post().Handler(s.createDepGraph)
	s.app.AddRoute("/depgraph/{id}").Version(1).Get().Handler(s.resolveDepGraph)
	s.app.AddRoute("/depgraph/{id}/nodes").Version(1).Post().Handler(s.addDepGraphNodes)
	s.app.AddRoute("/depgraph/{id}/nodes").Version(1).Get().Handler(s.getDepGraphNodes)
	s.app.AddRoute("/depgraph/{id}/edges").Version(1).Post().Handler(s.addDepGraphEdges)
	s.app.AddRoute("/depgraph/{id}/edges").Version(1).Get().Handler(s.getDepGraphEdges)

	s.app.AddRoute("/perf/{id}").Version(1).Get().RouteHandler(makeGetPerfById(s.sc))
	s.app.AddRoute("/perf/task_id/{task_id}").Version(1).Get().RouteHandler(makeGetPerfByTaskId(s.sc))
	s.app.AddRoute("/perf/task_name/{task_name}").Version(1).Get().RouteHandler(makeGetPerfByTaskName(s.sc))
	s.app.AddRoute("/perf/version/{version}").Version(1).Get().RouteHandler(makeGetPerfByVersion(s.sc))
	s.app.AddRoute("/perf/children/{id}").Version(1).Get().RouteHandler(makeGetPerfChildren(s.sc))
}
