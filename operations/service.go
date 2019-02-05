package operations

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest"
	"github.com/evergreen-ci/cedar/rpc"
	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/recovery"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

// Service returns the ./cedar client sub-command object, which is
// responsible for starting the service.
func Service() cli.Command {
	const (
		localQueueFlag       = "localQueue"
		servicePortFlag      = "port"
		envVarRPCPort        = "CEDAR_RPC_PORT"
		envVarRPCHost        = "CEDAR_RPC_HOST"
		envVarRPCCertPath    = "CEDAR_RPC_CERT"
		envVarRPCCertKeyPath = "CEDAR_RPC_KEY"
		envVarRPCCAPath      = "CEDAR_RPC_CA"
		envVarRESTPort       = "CEDAR_REST_PORT"

		rpcHostFlag        = "rpcHost"
		rpcPortFlag        = "rpcPort"
		rpcCertPathFlag    = "rpcCertPath"
		rpcCAPathFlag      = "rpcCAPath"
		rpcCertKeyPathFlag = "rpcKeyPath"
	)

	return cli.Command{
		Name:  "service",
		Usage: "run the cedar api service",
		Flags: mergeFlags(
			baseFlags(),
			dbFlags(
				cli.StringFlag{
					Name:   rpcCertPathFlag,
					Usage:  "path to the rpc service certificate",
					EnvVar: envVarRPCCertPath,
				},
				cli.StringFlag{
					Name:   rpcCAPathFlag,
					Usage:  "path to the rpc service ca cert",
					EnvVar: envVarRPCCAPath,
				},
				cli.StringFlag{
					Name:   rpcCertKeyPathFlag,
					Usage:  "path to the prc service key",
					EnvVar: envVarRPCCertKeyPath,
				},
				cli.BoolFlag{
					Name:  localQueueFlag,
					Usage: "uses a locally-backed queue rather than MongoDB",
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
			port := c.Int(servicePortFlag)

			rpcCertPath := c.String(rpcCertPathFlag)
			rpcCertKeyPath := c.String(rpcCertKeyPathFlag)
			rpcCAPath := c.String(rpcCAPathFlag)
			rpcHost := c.String(rpcHostFlag)
			rpcPort := c.Int(rpcPortFlag)
			rpcAddr := fmt.Sprintf("%s:%d", rpcHost, rpcPort)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go signalListener(ctx, cancel)
			env := cedar.GetEnvironment()
			sc := newServiceConf(workers, runLocal, mongodbURI, bucket, dbName)

			if err := sc.setup(ctx, env); err != nil {
				return errors.WithStack(err)
			}

			conf := &model.CedarConfig{}
			conf.Setup(env)
			if err := conf.Find(); err != nil {
				return errors.Wrap(err, "problem getting application configuration")
			}

			///////////////////////////////////
			//
			// starting rest service
			//
			service := &rest.Service{
				Port:        port,
				Prefix:      "rest",
				Environment: env,
				Conf:        conf,
				CertPath:    filepath.Dir(rpcCertPath),
				RootCAName:  strings.TrimSuffix(filepath.Base(rpcCAPath), filepath.Ext(rpcCAPath)),
				RPCServers:  conf.Service.AppServers,
			}

			restWait, err := service.Start(ctx)
			if err != nil {
				return errors.Wrap(err, "problem starting")
			}

			adminWait, err := adminService.BackgroundRun(ctx)
			if err != nil {
				return errors.Wrap(err, "problem starting admin service")
			}

			///////////////////////////////////
			//
			// starting grpc
			//

			rpcSrv, err := rpc.GetServer(env, rpc.CertConfig{
				Cert: rpcCertPath,
				Key:  rpcCertKeyPath,
				CA:   rpcCAPath,
			})

			if err != nil {
				return errors.WithStack(err)
			}

			rpcWait, err := rpc.RunServer(ctx, rpcSrv, rpcAddr)
			if err != nil {
				return errors.WithStack(err)
			}

			ctx, cancel = context.WithCancel(context.Background())
			defer cancel()

			adminWait(ctx)
			restWait(ctx)
			rpcWait(ctx)

			return nil
		},
	}
}

func getAdminService(env environment) (*gimplet.APIApp, error) {
	conf := env.GetConfiguration()

	MakeDBQueueState(cedar.QueueName, 

	reporter := 
	app := gimlet.NewApp()
	app.AddMiddleware(gimlet.MakeRecoveryLogger())
	app.Merge(gimlet.GetPProfApp())
	gimlet.MergeApplications(app, adminService)
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
