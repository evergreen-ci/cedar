package operations

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/rest"
	"github.com/evergreen-ci/cedar/rpc"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/recovery"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	grpc "google.golang.org/grpc"
)

// Service returns the ./cedar client sub-command object, which is
// responsible for starting the service.
func Service() cli.Command {
	const (
		localQueueFlag  = "localQueue"
		servicePortFlag = "port"
		envVarGRPCPort  = "CEDAR_GRPC_PORT"
		envVarGRPCHost  = "CEDAR_GRPC_HOST"
		envVarRESTPort  = "CEDAR_REST_PORT"

		grpcHostFlag = "rpcHost"
		grpcPortFlag = "rpcPort"
	)

	return cli.Command{
		Name:  "service",
		Usage: "run the cedar api service",
		Flags: mergeFlags(
			baseFlags(),
			dbFlags(
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
					Name:   grpcPortFlag,
					Usage:  "port for the grpc service",
					EnvVar: envVarGRPCPort,
					Value:  2289,
				},
				cli.StringFlag{
					Name:   grpcHostFlag,
					Usage:  "hostName for the grpc service",
					EnvVar: envVarGRPCHost,
					Value:  "localhost",
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

			restHost := c.String(grpcHostFlag)
			restPort := c.Int(grpcPortFlag)
			rpcAddr := fmt.Sprintf("%s:%d", restHost, restPort)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go signalListener(ctx, cancel)
			env := cedar.GetEnvironment()

			if err := configure(env, workers, runLocal, mongodbURI, bucket, dbName); err != nil {
				return errors.WithStack(err)
			}

			///////////////////////////////////
			//
			// starting rest service
			//
			service := &rest.Service{
				Port:        port,
				Prefix:      "rest",
				Environment: env,
			}
			if err := service.Validate(); err != nil {
				return errors.Wrap(err, "problem validating service")
			}

			restWait := make(chan struct{})
			go func() {
				defer close(restWait)
				defer recovery.LogStackTraceAndContinue("running rest service")
				grip.Noticef("starting cedar REST service on :%d", port)
				grip.Alert(errors.Wrap(service.Start(ctx), "problem running rest service"))
			}()

			///////////////////////////////////
			//
			// starting grpc
			//
			rpcSrv := grpc.NewServer()
			rpc.AttachService(env, rpcSrv)

			lis, err := net.Listen("tcp", rpcAddr)
			if err != nil {
				return errors.WithStack(err)
			}

			go func() {
				defer recovery.LogStackTraceAndExit("running rest service")
				grip.Warning(rpcSrv.Serve(lis))
			}()

			rpcWait := make(chan struct{})
			go func() {
				defer close(rpcWait)
				defer recovery.LogStackTraceAndContinue("waiting for the rpc service")
				<-ctx.Done()
				rpcSrv.Stop()
				grip.Info("jasper rpc service terminated")
			}()

			<-restWait
			<-rpcWait

			return nil
		},
	}
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
