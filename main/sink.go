package main

import (
	"os"

	mgo "gopkg.in/mgo.v2"

	"github.com/mongodb/amboy/queue"
	"github.com/mongodb/amboy/queue/driver"
	"github.com/pkg/errors"
	"github.com/tychoish/grip"
	"github.com/tychoish/sink"
	"github.com/tychoish/sink/rest"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
)

func main() {
	// this is where the main action of the program starts. The
	// command line interface is managed by the cli package and
	// its objects/structures. This, plus the basic configuration
	// in buildApp(), is all that's necessary for bootstrapping the
	// environment.
	app := buildApp()
	err := app.Run(os.Args)
	grip.CatchEmergencyFatal(err)
}

func buildApp() *cli.App {
	app := cli.NewApp()

	app.Name = "sink"
	app.Usage = "a data processing API"
	app.Version = "0.0.1-pre"

	app.Commands = []cli.Command{
		service(),
	}

	// These are global options. Use this to configure logging or
	// other options independent from specific sub commands.
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "level",
			Value: "info",
			Usage: "Specify lowest visible loglevel as string: 'emergency|alert|critical|error|warning|notice|info|debug'",
		},
	}

	app.Before = func(c *cli.Context) error {
		loggingSetup(app.Name, c.String("level"))
		return nil
	}

	return app
}

func service() cli.Command {
	return cli.Command{
		Name:  "service",
		Usage: "run the sink api service",
		Flags: []cli.Flag{
			cli.IntFlag{
				Name:  "workers, jobs",
				Usage: "specify the number of worker jobs this process will have",
				Value: 2,
			},
			cli.BoolFlag{
				Name:  "localQueue",
				Usage: "uses a locally-backed queue rather than MongoDB",
			},
			cli.IntFlag{
				Name:   "port, p",
				Usage:  "specify a port to run the service on",
				Value:  3000,
				EnvVar: "SINK_SERVICE_PORT",
			},
			cli.StringFlag{
				Name:   "dbUri, d",
				Usage:  "specify a mongodb connection string",
				Value:  "mongodb://localhost:27017",
				EnvVar: "SINK_MONGODB_URL",
			},
			cli.StringFlag{
				Name:   "bucket",
				Usage:  "specify a bucket name to use for storing data in s3",
				EnvVar: "SINK_BUCKET_NAME",
				Value:  "build-curator-testing",
			},
		},
		Action: func(c *cli.Context) error {
			ctx := context.Background()
			workers := c.Int("workers")
			mongodbURI := c.String("dbUri")
			runLocal := c.BoolFlag("localQueue")

			////////////////////////////////////////////////////////////////////////
			//
			// set up the application globals. this should
			if runLocal {
				q := queue.NewLocalUnordered(workers)
				grip.Infof("configured local queue with %d workers", workers)
				if err := sink.SetQueue(q); err != nil {
					return errors.Wrap(err, "problem configuring queue")
				}
			} else {
				q := queue.NewRemoteUnordered(workers)
				opts := driver.MongoDBOptions{
					URI:      mongodbURI,
					DB:       sink.DBName,
					Priority: true,
				}

				if err := sink.SetDriverOpts(queueName, opts); err != nil {
					return errors.Wrap(err, "problem caching queue driver options")
				}

				mongoDriver := driver.NewMongoDB(queueName, opts)
				if err := q.SetDriver(mongoDriver); err != nil {
					return errors.Wrap(err, "problem configuring driver")
				}

				if err := sink.SetQueue(q); err != nil {
					return errors.Wrap(err, "problem caching queue")
				}

				grip.Infof("configured a remote mongodb-backed queue "+
					"[db=%s, prefix=%s, priority=%t]", sink.DBName, queueName, true)

			}

			sink.SetConf(&sink.SinkConfiguration{
				BucketName: c.String("bucket"),
			})

			// create and cache a db session for use in tasks
			session, err := mgo.Dial(mongodbURI)
			if err != nil {
				return errors.Wrapf(err, "could not connect to db %s", mongodbURI)
			}
			if err := sink.SetMgoSession(session); err != nil {
				return errors.Wrap(err, "problem caching DB session")
			}

			////////////////////////////////////////////////////////////////////////
			//
			// Setup and run service.
			service := &rest.Service{
				Port: c.Int("port"),
			}

			if err := service.Validate(); err != nil {
				return errors.Wrap(err, "problem validating service")
			}

			return errors.WithStack(service.Start(ctx))
		},
	}
}

// logging setup is separate to make it unit testable
func loggingSetup(name, level string) {
	grip.SetName(name)
	grip.SetThreshold(level)
}
