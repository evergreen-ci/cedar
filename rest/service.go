package rest

import (
	"context"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/certdepot"
	"github.com/evergreen-ci/gimlet"
	"github.com/evergreen-ci/gimlet/cached"
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
	umConf gimlet.UserMiddlewareConfiguration
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

	s.umConf = cedar.GetUserMiddlewareConfiguration()
	if err = s.umConf.Validate(); err != nil {
		return errors.New("programmer error; invalid user manager configuration")
	}

	if err = s.setupUserAuth(); err != nil {
		return errors.Wrap(err, "setting up auth")
	}

	if s.queue == nil {
		s.queue = s.Environment.GetRemoteQueue()
		if s.queue == nil {
			return errors.New("no queue defined")
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
		return errors.New("no certificate depot provided")
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

func (s *Service) setupUserAuth() error {
	var readOnly []gimlet.UserManager
	var readWrite []gimlet.UserManager
	if s.Conf.ServiceAuth.Enabled {
		usrMngr, err := s.setupServiceAuth()
		if err != nil {
			return errors.Wrap(err, "setting up service user auth")
		}
		readOnly = append(readOnly, usrMngr)
	}
	if s.Conf.LDAP.URL != "" {
		usrMngr, err := s.setupLDAPAuth()
		if err != nil {
			return errors.Wrap(err, "setting up LDAP user auth")
		}
		readWrite = append(readWrite, usrMngr)
	}
	if s.Conf.NaiveAuth.AppAuth {
		usrMngr, err := s.setupNaiveAuth()
		if err != nil {
			return errors.Wrap(err, "setting up naive user auth")
		}
		readOnly = append(readOnly, usrMngr)
	}

	if len(readOnly)+len(readWrite) == 0 {
		return errors.New("no user authentication method could be set up")
	}

	// Using multiple read-only user managers is only a temporary change to
	// migrate off of the dependence on the LDAP manager.
	s.UserManager = gimlet.NewMultiUserManager(readWrite, readOnly)

	return nil
}

func (s *Service) setupServiceAuth() (gimlet.UserManager, error) {
	opts := usercache.ExternalOptions{
		PutUserGetToken: func(gimlet.User) (string, error) {
			return "", errors.New("cannot put new users in DB")
		},
		GetUserByToken: func(string) (gimlet.User, bool, error) {
			return nil, false, errors.New("cannot get user by login token")
		},
		ClearUserToken: func(gimlet.User, bool) error {
			return errors.New("cannot clear user login token")
		},
		GetUserByID: func(id string) (gimlet.User, bool, error) {
			var user gimlet.User
			user, _, err := model.GetUser(id)
			if err != nil {
				return nil, false, errors.Errorf("finding user")
			}
			return user, true, nil
		},
		GetOrCreateUser: func(u gimlet.User) (gimlet.User, error) {
			var user gimlet.User
			user, _, err := model.GetUser(u.Username())
			if err != nil {
				return nil, errors.Wrap(err, "failed to find user and cannot create new one")
			}
			return user, nil
		},
	}

	cache, err := usercache.NewExternal(opts)
	if err != nil {
		return nil, errors.Wrap(err, "setting up user cache backed by DB")
	}
	usrMngr, err := cached.NewUserManager(cache)
	if err != nil {
		return nil, errors.Wrap(err, "creating user manager backed by DB")
	}

	return usrMngr, nil
}

func (s *Service) setupLDAPAuth() (gimlet.UserManager, error) {
	usrMngr, err := ldap.NewUserService(ldap.CreationOpts{
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
		return nil, errors.Wrap(err, "setting up ldap user manager")
	}
	return usrMngr, nil
}

func (s *Service) setupNaiveAuth() (gimlet.UserManager, error) {
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
	usrMngr, err := gimlet.NewBasicUserManager(users, nil)
	if err != nil {
		return nil, errors.Wrap(err, "setting up basic user manager")
	}
	return usrMngr, nil
}

func (s *Service) Start(ctx context.Context) (gimlet.WaitFunc, error) {
	if err := s.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid application")
	}

	s.addMiddleware()
	s.addRoutes()

	if err := s.queue.Start(ctx); err != nil {
		return nil, errors.Wrap(err, "starting queue")
	}

	if err := s.app.Resolve(); err != nil {
		return nil, errors.Wrap(err, "resolving routes")
	}

	wait, err := s.app.BackgroundRun(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "starting service")
	}

	return wait, nil
}

func (s *Service) addMiddleware() {
	s.app.AddMiddleware(gimlet.MakeRecoveryLogger())
	s.app.AddMiddleware(gimlet.UserMiddleware(s.UserManager, s.umConf))
	s.app.AddMiddleware(gimlet.NewAuthenticationHandler(gimlet.NewBasicAuthenticator(nil, nil), s.UserManager))

	if s.Conf.Service.CORSOrigins != nil {
		s.app.AddMiddleware(cors.New(cors.Options{
			AllowedOrigins:   s.Conf.Service.CORSOrigins,
			AllowCredentials: true,
		}))
	}
}

func (s *Service) addRoutes() {
	checkUser := gimlet.NewRequireAuthHandler()
	checkDepot := newCertCheckDepotMiddleware(s.Depot == nil)
	evgAuthReadLogByID := newEvgAuthReadLogByIDMiddleware(s.sc, &s.Conf.Evergreen)
	evgAuthReadLogByTaskID := newEvgAuthReadLogByTaskIDMiddleware(s.sc, &s.Conf.Evergreen)

	s.app.AddRoute("/admin/status").Version(1).Get().Handler(s.statusHandler)
	s.app.AddRoute("/admin/status/event/{id}").Version(1).Get().Wrap(checkUser).Handler(s.getSystemEvent)
	s.app.AddRoute("/admin/status/event/{id}/acknowledge").Version(1).Get().Wrap(checkUser).Handler(s.acknowledgeSystemEvent)
	s.app.AddRoute("/admin/status/events/{level}").Version(1).Get().Wrap(checkUser).Handler(s.getSystemEvents)
	s.app.AddRoute("/admin/service/flag/{flagName}/enabled").Version(1).Post().Wrap(checkUser).Handler(s.setServiceFlagEnabled)
	s.app.AddRoute("/admin/service/flag/{flagName}/disabled").Version(1).Post().Wrap(checkUser).Handler(s.setServiceFlagDisabled)
	s.app.AddRoute("/admin/ca").Version(1).Get().Wrap(checkDepot).Handler(s.fetchRootCert)
	s.app.AddRoute("/admin/users/certificate").Version(1).Post().Get().Wrap(checkDepot).Handler(s.fetchUserCert)
	s.app.AddRoute("/admin/users/certificate/key").Version(1).Post().Get().Wrap(checkDepot).Handler(s.fetchUserCertKey)
	s.app.AddRoute("/admin/perf/change_points").Version(1).Post().Wrap(checkUser).RouteHandler(makePerfSignalProcessingRecalculate(s.sc))

	s.app.AddRoute("/simple_log/{id}").Version(1).Post().Wrap(checkUser).Handler(s.simpleLogIngestion)
	s.app.AddRoute("/simple_log/{id}").Version(1).Get().Handler(s.simpleLogRetrieval)
	s.app.AddRoute("/simple_log/{id}/text").Version(1).Get().Handler(s.simpleLogGetText)
	s.app.AddRoute("/system_info").Version(1).Post().Wrap(checkUser).Handler(s.recieveSystemInfo)
	s.app.AddRoute("/system_info/host/{host}").Version(1).Post().Wrap(checkUser).Handler(s.fetchSystemInfo)

	s.app.AddRoute("/perf/{id}").Version(1).Get().RouteHandler(makeGetPerfById(s.sc))
	s.app.AddRoute("/perf/{id}").Version(1).Delete().Wrap(checkUser).RouteHandler(makeRemovePerfById(s.sc))
	s.app.AddRoute("/perf/children/{id}").Version(1).Get().RouteHandler(makeGetPerfChildren(s.sc))
	s.app.AddRoute("/perf/task_id/{task_id}").Version(1).Get().RouteHandler(makeGetPerfByTaskId(s.sc))
	s.app.AddRoute("/perf/task_name/{task_name}").Version(1).Get().RouteHandler(makeGetPerfByTaskName(s.sc))
	s.app.AddRoute("/perf/version/{version}").Version(1).Get().RouteHandler(makeGetPerfByVersion(s.sc))

	s.app.AddRoute("/buildlogger/{id}").Version(1).Get().Wrap(evgAuthReadLogByID).RouteHandler(makeGetLogByID(s.sc))
	s.app.AddRoute("/buildlogger/{id}/meta").Version(1).Get().Wrap(evgAuthReadLogByID).RouteHandler(makeGetLogMetaByID(s.sc))
	s.app.AddRoute("/buildlogger/task_id/{task_id}").Version(1).Get().Wrap(evgAuthReadLogByTaskID).RouteHandler(makeGetLogByTaskID(s.sc))
	s.app.AddRoute("/buildlogger/task_id/{task_id}/meta").Version(1).Get().Wrap(evgAuthReadLogByTaskID).RouteHandler(makeGetLogMetaByTaskID(s.sc))
	s.app.AddRoute("/buildlogger/task_id/{task_id}/group/{group_id}").Version(0).Get().Wrap(evgAuthReadLogByTaskID).RouteHandler(makeGetLogGroupByTaskID(s.sc))
	s.app.AddRoute("/buildlogger/test_name/{task_id}/{test_name}").Version(1).Get().Wrap(evgAuthReadLogByTaskID).RouteHandler(makeGetLogByTestName(s.sc))
	s.app.AddRoute("/buildlogger/test_name/{task_id}/{test_name}/meta").Version(1).Get().Wrap(evgAuthReadLogByTaskID).RouteHandler(makeGetLogMetaByTestName(s.sc))
	s.app.AddRoute("/buildlogger/test_name/{task_id}/{test_name}/group/{group_id}").Version(1).Get().Wrap(evgAuthReadLogByTaskID).RouteHandler(makeGetLogGroupByTestName(s.sc))

	s.app.AddRoute("/test_results/task_id/{task_id}").Version(1).Get().RouteHandler(makeGetTestResultsByTaskID(s.sc))
	s.app.AddRoute("/test_results/display_task_id/{display_task_id}").Version(1).Get().RouteHandler(makeGetTestResultsByDisplayTaskID(s.sc))
	s.app.AddRoute("/test_results/test_name/{task_id}/{test_name}").Version(1).Get().RouteHandler(makeGetTestResultByTestName(s.sc))

	s.app.AddRoute("/historical_test_data/{project_id}").Version(1).Get().RouteHandler(makeGetHistoricalTestData(s.sc))

	s.app.AddRoute("/system_metrics/type/{task_id}/{type}").Version(1).Get().RouteHandler(makeGetSystemMetricsByType(s.sc))
}
