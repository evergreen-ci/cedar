package operations

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/certdepot"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest"
	"github.com/evergreen-ci/cedar/rpc"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/recovery"
	"github.com/pkg/errors"
	"github.com/square/certstrap/depot"
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

		rpcHostFlag = "rpcHost"
		rpcPortFlag = "rpcPort"
		rpcTLSFlag  = "rpcTLS"
	)

	return cli.Command{
		Name:  "service",
		Usage: "run the cedar api service",
		Flags: mergeFlags(
			baseFlags(),
			dbFlags(
				cli.BoolFlag{
					Name:  rpcTLSFlag,
					Usage: "specify whether to use TLS over rpc",
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

			rpcTLS := c.Bool(rpcTLSFlag)
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

			var d depot.Depot
			var err error
			if rpcTLS {
				d, err = setupDepot(conf.CertDepot)
				if err != nil {
					return errors.Wrap(err, "problem setting up the certificate depot")
				}
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
				RPCServers:  conf.Service.AppServers,
			}
			if rpcTLS {
				service.Depot = d
				service.CAName = conf.CertDepot.CAName
				service.ServiceName = conf.CertDepot.ServiceName
			}

			restWait, err := service.Start(ctx)
			if err != nil {
				return errors.Wrap(err, "problem starting")
			}

			///////////////////////////////////
			//
			// starting grpc
			//

			rpcSrv, err := rpc.GetServer(env, rpc.CertConfig{
				TLS:         rpcTLS,
				Depot:       d,
				CAName:      conf.CertDepot.CAName,
				ServiceName: conf.CertDepot.ServiceName,
			})

			if err != nil {
				return errors.WithStack(err)
			}

			rpcWait, err := rpc.RunServer(ctx, rpcSrv, rpcAddr)
			if err != nil {
				return errors.WithStack(err)
			}

			restWait()
			rpcWait()

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

func setupDepot(conf model.CertDepotConfig) (depot.Depot, error) {
	if conf.CAName == "" {
		return nil, errors.New("must proivde the name of the CA!")
	}
	if conf.ServiceName == "" {
		return nil, errors.New("must proivde the name of the service!")
	}

	var d depot.Depot
	var err error
	if conf.FileDepot {
		d, err = depot.NewFileDepot(conf.DepotName)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	} else {
		env := cedar.GetEnvironment()
		envConf, err := env.GetConf()
		if err != nil {
			return nil, errors.Wrap(err, "problem getting environemnt config")
		}

		opts := certdepot.MgoCertDepotOptions{
			MongoDBURI:           envConf.MongoDBURI,
			MongoDBDialTimeout:   envConf.MongoDBDialTimeout,
			MongoDBSocketTimeout: envConf.SocketTimeout,
			DatabaseName:         conf.DepotName,
			ExpireAfter:          conf.ExpireAfter,
		}
		d, err = certdepot.NewMgoCertDepot(opts)
		if err != nil {
			return nil, err
		}
	}

	if !depot.CheckCertificate(d, conf.CAName) {
		if err = createCA(d, conf); err != nil {
			return nil, errors.WithStack(err)
		}
	} else if !depot.CheckCertificate(d, conf.ServiceName) {
		if err = createServerCert(d, conf); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return d, nil
}

func createCA(d depot.Depot, conf model.CertDepotConfig) error {
	opts := certdepot.CertificateOptions{
		CommonName: conf.CAName,
	}

	if err := opts.Init(d); err != nil {
		return err
	}
	if err := createServerCert(d, conf); err != nil {
		return err
	}

	return nil
}

func createServerCert(d depot.Depot, conf model.CertDepotConfig) error {
	opts := certdepot.CertificateOptions{
		CommonName: conf.ServiceName,
		CA:         conf.CAName,
	}

	if err := opts.CertRequest(d); err != nil {
		return err
	}
	if err := opts.Sign(d); err != nil {
		return err
	}

	return nil
}
