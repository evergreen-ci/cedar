package operations

import (
	"github.com/pkg/errors"
	"github.com/tychoish/grip"
	"github.com/tychoish/sink/rest"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
)

func Service() cli.Command {
	return cli.Command{
		Name:  "service",
		Usage: "run the sink api service",
		Flags: baseFlags(
			cli.BoolFlag{
				Name:  "localQueue",
				Usage: "uses a locally-backed queue rather than MongoDB",
			},
			cli.IntFlag{
				Name:   "port, p",
				Usage:  "specify a port to run the service on",
				Value:  3000,
				EnvVar: "SINK_SERVICE_PORT",
			}),
		Action: func(c *cli.Context) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			workers := c.Int("workers")
			mongodbURI := c.String("dbUri")
			runLocal := c.Bool("localQueue")
			bucket := c.String("bucket")
			dbName := c.String("dbName")

			if err := configure(workers, runLocal, mongodbURI, bucket, dbName); err != nil {
				return errors.WithStack(err)
			}

			if err := backgroundJobs(ctx); err != nil {
				return errors.Wrap(err, "problem starting background jobs")
			}

			service := &rest.Service{
				Port: c.Int("port"),
			}

			if err := service.Validate(); err != nil {
				return errors.Wrap(err, "problem validating service")
			}

			if err := service.Start(ctx); err != nil {
				return errors.Wrap(err, "problem starting services")
			}

			grip.Noticef("starting sink service on :%d", c.Int("port"))
			service.Run()
			grip.Info("completed service, terminating.")
			return nil
		},
	}

}
