package operations

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest"
	"github.com/evergreen-ci/cedar/rpc"
	"github.com/evergreen-ci/cedar/units"
	"github.com/evergreen-ci/certdepot"
	"github.com/evergreen-ci/gimlet"
	amboyRest "github.com/mongodb/amboy/rest"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/recovery"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

// Service returns the ./cedar client sub-command object, which is
// responsible for starting the service.
func Service() cli.Command {
	const (
		localQueueFlag          = "localQueue"
		servicePortFlag         = "port"
		envVarRPCPort           = "CEDAR_RPC_PORT"
		envVarRPCHost           = "CEDAR_RPC_HOST"
		envVarRESTPort          = "CEDAR_REST_PORT"
		disableLocalLoggingFlag = "disableLocalLogging"
		disableLocalLoggingEnv  = "DISABLE_LOCAL_LOGGING"

		rpcHostFlag     = "rpcHost"
		rpcPortFlag     = "rpcPort"
		rpcTLSFlag      = "rpcTLS"
		rpcUserAuthFlag = "rpcUserAuth"
	)

	return cli.Command{
		Name:  "service",
		Usage: "run the Cedar API service",
		Flags: mergeFlags(
			baseFlags(),
			dbFlags(
				cli.BoolFlag{
					Name:  rpcTLSFlag,
					Usage: "specify whether to enable TLS over rpc",
				},
				cli.BoolFlag{
					Name:  rpcUserAuthFlag,
					Usage: "specify whether to enable user auth over rpc",
				},
				cli.BoolFlag{
					Name:  localQueueFlag,
					Usage: "uses a locally-backed queue rather than MongoDB",
				},
				cli.BoolFlag{
					Name:   disableLocalLoggingFlag,
					Usage:  "disable logging to stdout",
					EnvVar: disableLocalLoggingEnv,
				},
				cli.IntFlag{
					Name:   joinFlagNames(servicePortFlag, "p"),
					Usage:  "specify a port to run the REST service on",
					Value:  3000,
					EnvVar: envVarRESTPort,
				},
				cli.IntFlag{
					Name:   rpcPortFlag,
					Usage:  "port for the grpc service",
					EnvVar: envVarRPCPort,
					Value:  2289,
				},
				cli.StringFlag{
					Name:   rpcHostFlag,
					Usage:  "hostName for the grpc service",
					EnvVar: envVarRPCHost,
					Value:  "0.0.0.0",
				},
			),
		),
		Action: func(c *cli.Context) error {
			workers := c.Int(numWorkersFlag)
			mongodbURI := c.String(dbURIFlag)
			runLocal := c.Bool(localQueueFlag)
			bucket := c.String(bucketNameFlag)
			dbName := c.String(dbNameFlag)
			dbCredFile := c.String(dbCredsFileFlag)
			disableLocalLogging := c.Bool(disableLocalLoggingFlag)
			port := c.Int(servicePortFlag)

			println(disableLocalLogging)
			return nil

			rpcTLS := c.Bool(rpcTLSFlag)
			rpcUserAuth := c.Bool(rpcUserAuthFlag)
			rpcHost := c.String(rpcHostFlag)
			rpcPort := c.Int(rpcPortFlag)
			rpcAddr := fmt.Sprintf("%s:%d", rpcHost, rpcPort)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go signalListener(ctx, cancel)

			sc := newServiceConf(workers, runLocal, mongodbURI, bucket, dbName, dbCredFile, disableLocalLogging)
			if err := sc.setup(ctx); err != nil {
				return errors.WithStack(err)
			}

			env := cedar.GetEnvironment()

			conf := &model.CedarConfig{}
			conf.Setup(env)
			if err := conf.Find(); err != nil {
				return errors.Wrap(err, "getting application configuration")
			}

			var d certdepot.Depot
			var err error
			if rpcTLS {
				d, err = certdepot.BootstrapDepotWithMongoClient(ctx, env.GetClient(), conf.CA.CertDepot)
				if err != nil {
					return errors.Wrap(err, "setting up the certificate depot")
				}
			}

			///////////////////////////////////
			//
			// starting REST service
			//
			service := &rest.Service{
				Port:        port,
				Prefix:      "rest",
				Environment: env,
				Conf:        conf,
				RPCServers:  conf.Service.AppServers,
			}
			if rpcTLS {
				service.Depot = d
			}

			restWait, err := service.Start(ctx)
			if err != nil {
				return errors.Wrap(err, "starting public REST service")
			}

			adminService, err := getAdminService(env)
			if err != nil {
				return errors.Wrap(err, "resolving admin REST interface")
			}

			adminWait, err := adminService.BackgroundRun(ctx)
			if err != nil {
				return errors.Wrap(err, "starting admin REST service")
			}

			///////////////////////////////////
			//
			// starting grpc
			//

			rpcSrv, err := rpc.GetServer(env, rpc.AuthConfig{
				TLS:         rpcTLS,
				UserAuth:    rpcUserAuth,
				Depot:       d,
				CAName:      conf.CA.CertDepot.CAName,
				ServiceName: conf.CA.CertDepot.ServiceName,
				UserManager: service.UserManager,
			})
			if err != nil {
				return errors.WithStack(err)
			}

			rpcWait, err := rpc.RunServer(ctx, rpcSrv, rpcAddr)
			if err != nil {
				return errors.WithStack(err)
			}

			if err := units.StartCrons(ctx, env, rpcTLS); err != nil {
				return errors.WithStack(err)
			}

			if c.String(dbURIFlag) != "mongodb://localhost:27017" {
				// Check indexes in production only.
				if err := model.CheckIndexes(ctx, env.GetDB(), model.GetRequiredIndexes()); err != nil {
					grip.Error(errors.Wrap(err, "missing expected DB indexes"))
				}
			}

			ctx, cancel = context.WithCancel(context.Background())
			defer cancel()
			env.RegisterCloser("web-services-closer", func(_ context.Context) error {
				cancel()
				return nil
			})

			adminWait(ctx)
			restWait(ctx)
			rpcWait(ctx)

			return nil
		},
	}
}

func getAdminService(env cedar.Environment) (*gimlet.APIApp, error) {
	app := gimlet.NewApp()

	if err := app.SetPort(2285); err != nil {
		return nil, errors.WithStack(err)
	}
	app.NoVersions = true

	app.AddMiddleware(gimlet.MakeRecoveryLogger())

	err := app.Merge(gimlet.GetPProfApp(), amboyRest.NewManagementService(env.GetRemoteManager()).App())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return app, nil
}

func signalListener(ctx context.Context, trigger context.CancelFunc) {
	defer recovery.LogStackTraceAndContinue("graceful shutdown")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM)

	select {
	case <-sigChan:
		grip.Debug("received signal")
	case <-ctx.Done():
		grip.Debug("context canceled")
	}

	trigger()
}
