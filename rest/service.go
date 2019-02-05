package rest

import (
	"context"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/gimlet"
	"github.com/evergreen-ci/gimlet/ldap"
	"github.com/mongodb/amboy"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"github.com/square/certstrap/depot"
)

type Service struct {
	Port        int
	Prefix      string
	Environment cedar.Environment
	Conf        *model.CedarConfig

	RPCServers []string
	CertPath   string
	RootCAName string

	// internal settings
	depot depot.Depot
	um    gimlet.UserManager
	queue amboy.Queue
	app   *gimlet.APIApp
	sc    data.Connector
}

func (s *Service) Validate() error {
	var err error

	if s.Environment == nil {
		return errors.New("must specify an environment")
	}

	if s.Conf == nil {
		return errors.New("must specify a non-nil config")
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

	if s.Conf.LDAP.URL != "" {
		s.um, err = ldap.NewUserService(ldap.CreationOpts{
			URL:           s.Conf.LDAP.URL,
			Port:          s.Conf.LDAP.Port,
			UserPath:      s.Conf.LDAP.UserPath,
			ServicePath:   s.Conf.LDAP.ServicePath,
			UserGroup:     s.Conf.LDAP.UserGroup,
			ServiceGroup:  s.Conf.LDAP.ServiceGroup,
			PutCache:      model.PutLoginCache,
			GetCache:      model.GetLoginCache,
			ClearCache:    model.ClearLoginCache,
			GetUser:       model.GetUser,
			GetCreateUser: model.GetOrAddUser,
		})
		if err != nil {
			return errors.Wrap(err, "problem setting up user manager")
		}
	}

	s.depot, err = depot.NewFileDepot(s.CertPath)
	grip.Warning(errors.Wrap(err, "no certificate depot constructed"))

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

		grip.Critical(message.WrapError(err, message.Fields{
			"op":   "finding local config",
			"addr": addr,
		}))

		s.RPCServers = []string{addr}
	}

	grip.Info(message.Fields{
		"message": "detected local rpc services",
		"service": s.RPCServers,
	})
	return nil
}

func (s *Service) Start(ctx context.Context) (gimlet.WaitFunc, error) {
	if err := s.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid application")
	}

	s.addMiddleware()
	s.addRoutes()

	if err := s.queue.Start(ctx); err != nil {
		return nil, errors.Wrap(err, "problem starting queue")
	}

	if err := s.app.Resolve(); err != nil {
		return nil, errors.Wrap(err, "problem resolving routes")
	}

	wait, err := s.app.BackgroundRun(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "problem starting service")
	}

	return wait, nil
}

func (s *Service) addMiddleware() {
	s.app.AddMiddleware(gimlet.MakeRecoveryLogger())

	s.app.AddMiddleware(gimlet.UserMiddleware(s.um, gimlet.UserMiddlewareConfiguration{
		CookieName:     cedar.AuthTokenCookie,
		HeaderKeyName:  cedar.APIKeyHeader,
		HeaderUserName: cedar.APIUserHeader,
	}))

	s.app.AddMiddleware(gimlet.NewAuthenticationHandler(gimlet.NewBasicAuthenticator(nil, nil), s.um))
}

func (s *Service) addRoutes() {
	checkUser := gimlet.NewRequireAuthHandler()

	s.app.AddRoute("/admin/status").Version(1).Get().Handler(s.statusHandler)
	s.app.AddRoute("/admin/status/event/{id}").Version(1).Get().Wrap(checkUser).Handler(s.getSystemEvent)
	s.app.AddRoute("/admin/status/event/{id}/acknowledge").Version(1).Get().Wrap(checkUser).Handler(s.acknowledgeSystemEvent)
	s.app.AddRoute("/admin/status/events/{level}").Version(1).Get().Wrap(checkUser).Handler(s.getSystemEvents)
	s.app.AddRoute("/admin/service/flag/{flagName}/enabled").Version(1).Post().Wrap(checkUser).Handler(s.setServiceFlagEnabled)
	s.app.AddRoute("/admin/service/flag/{flagName}/disabled").Version(1).Post().Wrap(checkUser).Handler(s.setServiceFlagDisabled)
	s.app.AddRoute("/admin/ca").Version(1).Get().Handler(s.fetchRootCert)
	s.app.AddRoute("/admin/users/key").Version(1).Get().Handler(s.fetchUserToken)
	s.app.AddRoute("/admin/users/certificate").Version(1).Get().Handler(s.fetchUserCert)
	s.app.AddRoute("/admin/users/certificate/key").Version(1).Get().Handler(s.fetchUserCertKey)

	s.app.AddRoute("/simple_log/{id}").Version(1).Post().Wrap(checkUser).Handler(s.simpleLogInjestion)
	s.app.AddRoute("/simple_log/{id}").Version(1).Get().Handler(s.simpleLogRetrieval)
	s.app.AddRoute("/simple_log/{id}/text").Version(1).Get().Handler(s.simpleLogGetText)
	s.app.AddRoute("/system_info").Version(1).Post().Wrap(checkUser).Handler(s.recieveSystemInfo)
	s.app.AddRoute("/system_info/host/{host}").Version(1).Post().Wrap(checkUser).Handler(s.fetchSystemInfo)

	s.app.AddRoute("/depgraph/{id}").Version(1).Post().Wrap(checkUser).Handler(s.createDepGraph)
	s.app.AddRoute("/depgraph/{id}").Version(1).Get().Wrap(checkUser).Handler(s.resolveDepGraph)
	s.app.AddRoute("/depgraph/{id}/nodes").Version(1).Post().Wrap(checkUser).Handler(s.addDepGraphNodes)
	s.app.AddRoute("/depgraph/{id}/nodes").Version(1).Get().Wrap(checkUser).Handler(s.getDepGraphNodes)
	s.app.AddRoute("/depgraph/{id}/edges").Version(1).Post().Wrap(checkUser).Handler(s.addDepGraphEdges)
	s.app.AddRoute("/depgraph/{id}/edges").Version(1).Get().Wrap(checkUser).Handler(s.getDepGraphEdges)

	s.app.AddRoute("/perf/{id}").Version(1).Get().RouteHandler(makeGetPerfById(s.sc))
	s.app.AddRoute("/perf/{id}").Version(1).Delete().Wrap(checkUser).RouteHandler(makeRemovePerfById(s.sc))
	s.app.AddRoute("/perf/task_id/{task_id}").Version(1).Get().RouteHandler(makeGetPerfByTaskId(s.sc))
	s.app.AddRoute("/perf/task_name/{task_name}").Version(1).Get().RouteHandler(makeGetPerfByTaskName(s.sc))
	s.app.AddRoute("/perf/version/{version}").Version(1).Get().RouteHandler(makeGetPerfByVersion(s.sc))
	s.app.AddRoute("/perf/children/{id}").Version(1).Get().RouteHandler(makeGetPerfChildren(s.sc))
}
