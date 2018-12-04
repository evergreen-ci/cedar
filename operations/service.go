package operations

import (
	"context"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/rest"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

// Service returns the ./cedar client sub-command object, which is
// responsible for starting the service.
func Service() cli.Command {
	const (
		localQueueFlag  = "localQueue"
		servicePortFlag = "port"
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
					Usage:  "specify a port to run the service on",
					Value:  3000,
					EnvVar: "CEDAR_SERVICE_PORT",
				})),
		Action: func(c *cli.Context) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			workers := c.Int(numWorkersFlag)
			mongodbURI := c.String(dbURIFlag)
			runLocal := c.Bool(localQueueFlag)
			bucket := c.String(bucketNameFlag)
			dbName := c.String(dbNameFlag)
			port := c.Int(servicePortFlag)

			env := cedar.GetEnvironment()

			if err := configure(env, workers, runLocal, mongodbURI, bucket, dbName); err != nil {
				return errors.WithStack(err)
			}

			service := &rest.Service{
				Port: port,
			}

			if err := service.Validate(); err != nil {
				return errors.Wrap(err, "problem validating service")
			}

			if err := service.Start(ctx); err != nil {
				return errors.Wrap(err, "problem starting services")
			}

			if err := backgroundJobs(ctx, env); err != nil {
				return errors.Wrap(err, "problem starting background jobs")
			}

			grip.Noticef("starting cedar service on :%d", port)
			service.Run(ctx)
			grip.Info("completed service, terminating.")
			return nil
		},
	}
}
