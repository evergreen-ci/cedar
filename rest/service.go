package rest

import (
	"context"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/certdepot"
	"github.com/evergreen-ci/gimlet"
	"github.com/evergreen-ci/gimlet/ldap"
	"github.com/evergreen-ci/gimlet/usercache"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/amboy"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"github.com/rs/cors"
)

type Service struct {
	Port        int
	Prefix      string
	Environment cedar.Environment
	Conf        *model.CedarConfig
	UserManager gimlet.UserManager

	RPCServers []string
	Depot      certdepot.Depot

	// internal settings
	umconf gimlet.UserMiddlewareConfiguration
	queue  amboy.Queue
	app    *gimlet.APIApp
	sc     data.Connector
}

func (s *Service) Validate() error {
	var err error

	if s.Environment == nil {
		return errors.New("must specify an environment")
	}

	if s.Conf == nil {
		return errors.New("must specify a non-nil config")
	}

	s.umconf = gimlet.UserMiddlewareConfiguration{
		HeaderKeyName:  cedar.APIKeyHeader,
		HeaderUserName: cedar.APIUserHeader,
		CookieName:     cedar.AuthTokenCookie,
		CookiePath:     "/",
		CookieTTL:      cedar.TokenExpireAfter,
	}
	if err = s.umconf.Validate(); err != nil {
		return errors.New("programmer error; invalid user manager configuration")
	}

	if s.queue == nil {
		s.queue = s.Environment.GetRemoteQueue()
		if s.queue == nil {
			return errors.New("no queue defined")
		}
	}

	if s.Conf.LDAP.URL != "" {
		s.UserManager, err = ldap.NewUserService(ldap.CreationOpts{
			URL:          s.Conf.LDAP.URL,
			Port:         s.Conf.LDAP.Port,
			UserPath:     s.Conf.LDAP.UserPath,
			ServicePath:  s.Conf.LDAP.ServicePath,
			UserGroup:    s.Conf.LDAP.UserGroup,
			ServiceGroup: s.Conf.LDAP.ServiceGroup,
			ExternalCache: &usercache.ExternalOptions{
				PutUserGetToken: model.PutLoginCache,
				GetUserByToken:  model.GetLoginCache,
				ClearUserToken:  model.ClearLoginCache,
				GetUserByID:     model.GetUser,
				GetOrCreateUser: model.GetOrAddUser,
			},
		})
		if err != nil {
			return errors.Wrap(err, "problem setting up ldap user manager")
		}
	} else if s.Conf.NaiveAuth.AppAuth {
		users := []gimlet.BasicUser{}
		for _, user := range s.Conf.NaiveAuth.Users {
			users = append(
				users,
				gimlet.BasicUser{
					ID:           user.ID,
					Name:         user.Name,
					EmailAddress: user.EmailAddress,
					Password:     user.Password,
					Key:          user.Key,
					AccessRoles:  user.AccessRoles,
				},
			)
		}
		s.UserManager, err = gimlet.NewBasicUserManager(users, nil)
		if err != nil {
			return errors.Wrap(err, "problem setting up basic user manager")
		}
	}

	if s.Conf.CA.SSLExpireAfter == 0 {
		s.Conf.CA.SSLExpireAfter = 48 * time.Hour
	}
	if s.Conf.CA.SSLRenewalBefore == 0 {
		s.Conf.CA.SSLRenewalBefore = 4 * time.Hour
	}

	if s.queue == nil {
		s.queue = s.Environment.GetRemoteQueue()
		if s.queue == nil {
			return errors.New("no queue defined")
		}
	}

	if s.Depot == nil {
		grip.Warning("no certificate depot provided")
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
		addr, err := utility.GetPublicIP()

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

	s.app.AddMiddleware(gimlet.UserMiddleware(s.UserManager, s.umconf))

	s.app.AddMiddleware(gimlet.NewAuthenticationHandler(gimlet.NewBasicAuthenticator(nil, nil), s.UserManager))

	if s.Conf.Service.CORSOrigins != nil {
		s.app.AddMiddleware(cors.New(cors.Options{
			AllowedOrigins: s.Conf.Service.CORSOrigins,
		}))
	}
}

func (s *Service) addRoutes() {
	checkUser := gimlet.NewRequireAuthHandler()
	evgAuthReadLogByID := NewEvgAuthReadLogByIDMiddleware(s.sc, &s.Conf.Evergreen)
	evgAuthReadLogByTaskID := NewEvgAuthReadLogByTaskIDMiddleware(s.sc, &s.Conf.Evergreen)

	s.app.AddRoute("/admin/status").Version(1).Get().Handler(s.statusHandler)
	s.app.AddRoute("/admin/status/event/{id}").Version(1).Get().Wrap(checkUser).Handler(s.getSystemEvent)
	s.app.AddRoute("/admin/status/event/{id}/acknowledge").Version(1).Get().Wrap(checkUser).Handler(s.acknowledgeSystemEvent)
	s.app.AddRoute("/admin/status/events/{level}").Version(1).Get().Wrap(checkUser).Handler(s.getSystemEvents)
	s.app.AddRoute("/admin/service/flag/{flagName}/enabled").Version(1).Post().Wrap(checkUser).Handler(s.setServiceFlagEnabled)
	s.app.AddRoute("/admin/service/flag/{flagName}/disabled").Version(1).Post().Wrap(checkUser).Handler(s.setServiceFlagDisabled)
	s.app.AddRoute("/admin/ca").Version(1).Get().Handler(s.fetchRootCert)
	s.app.AddRoute("/admin/users/key").Version(1).Post().Get().Handler(s.fetchUserToken)
	s.app.AddRoute("/admin/users/certificate").Version(1).Post().Get().Handler(s.fetchUserCert)
	s.app.AddRoute("/admin/users/certificate/key").Version(1).Post().Get().Handler(s.fetchUserCertKey)

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
	s.app.AddRoute("/perf/change_points").Version(1).Post().Wrap(checkUser).RouteHandler(makePerfSignalProcessingRecalculate(s.sc))
	s.app.AddRoute("/perf/change_points/triage/mark").Version(1).Post().Wrap(checkUser).RouteHandler(makePerfChangePointTriageMarkHandler(s.sc))
	s.app.AddRoute("/perf/project/{projectID}/change_points_by_version").Version(1).Get().RouteHandler(makeGetChangePointsByVersion(s.sc))

	s.app.AddRoute("/buildlogger/{id}").Version(1).Get().Wrap(evgAuthReadLogByID).RouteHandler(makeGetLogByID(s.sc))
	s.app.AddRoute("/buildlogger/{id}/meta").Version(1).Get().Wrap(evgAuthReadLogByID).RouteHandler(makeGetLogMetaByID(s.sc))
	s.app.AddRoute("/buildlogger/task_id/{task_id}").Version(1).Get().Wrap(evgAuthReadLogByTaskID).RouteHandler(makeGetLogByTaskID(s.sc))
	s.app.AddRoute("/buildlogger/task_id/{task_id}/meta").Version(1).Get().Wrap(evgAuthReadLogByTaskID).RouteHandler(makeGetLogMetaByTaskID(s.sc))
	s.app.AddRoute("/buildlogger/test_name/{task_id}/{test_name}").Version(1).Get().Wrap(evgAuthReadLogByTaskID).RouteHandler(makeGetLogByTestName(s.sc))
	s.app.AddRoute("/buildlogger/test_name/{task_id}/{test_name}/meta").Version(1).Get().Wrap(evgAuthReadLogByTaskID).RouteHandler(makeGetLogMetaByTestName(s.sc))
	s.app.AddRoute("/buildlogger/test_name/{task_id}/{test_name}/group/{group_id}").Version(1).Get().Wrap(evgAuthReadLogByTaskID).RouteHandler(makeGetLogGroup(s.sc))
}
