package operations

import (
	"context"
	"strings"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/amboy"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

// Worker returns the ./cedar worker command, which is responsible for
// starting a cedar service that does *not* host the REST API, and only
// processes jobs from the queue.
func Worker() cli.Command {
	return cli.Command{
		Name: "worker",
		Usage: strings.Join([]string{
			"run a data processing node without a web front-end",
			"runs jobs until there is no more pending work, or 1 minute, whichever is longer",
		}, "\n\t"),
		Flags: mergeFlags(baseFlags(), dbFlags()),
		Action: func(c *cli.Context) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			workers := c.Int(numWorkersFlag)
			mongodbURI := c.String(dbURIFlag)
			bucket := c.String(bucketNameFlag)
			dbName := c.String(dbNameFlag)
			dbCredFile := c.String(dbCredsFileFlag)
			dbConfigurationCollection := c.String(dbConfigurationCollectionFlag)

			sc := newServiceConf(workers, false, mongodbURI, bucket, dbName, dbCredFile, false, dbConfigurationCollection)
			if err := sc.setup(ctx); err != nil {
				return errors.WithStack(err)
			}

			env := cedar.GetEnvironment()
			q := env.GetRemoteQueue()
			if err := q.Start(ctx); err != nil {
				return errors.Wrap(err, "starting queue")
			}

			time.Sleep(time.Minute)
			grip.Info(q.Stats(ctx))
			amboy.WaitInterval(ctx, q, time.Second)
			grip.Notice("no pending work; shutting worker down.")

			return nil
		},
	}
}
