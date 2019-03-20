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

			sc := newServiceConf(workers, false, mongodbURI, bucket, dbName)
			if err := sc.setup(ctx); err != nil {
				return errors.WithStack(err)
			}

			env := cedar.GetEnvironment()
			q, err := env.GetRemoteQueue()
			if err != nil {
				return errors.Wrap(err, "problem getting queue")
			}

			if err = q.Start(ctx); err != nil {
				return errors.Wrap(err, "problem starting queue")
			}

			time.Sleep(time.Minute)
			grip.Debug(q.Stats())
			amboy.WaitCtxInterval(ctx, q, time.Second)
			grip.Notice("no pending work; shutting worker down.")

			return nil
		},
	}
}
