package operations

import (
	"fmt"
	"time"

	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/cost"
	"github.com/evergreen-ci/sink/model"
	"github.com/evergreen-ci/sink/units"
	"github.com/mongodb/amboy"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
)

// Cost returns the entry point for the ./sink spend sub-command,
// which has required flags.
func Cost() cli.Command {
	return cli.Command{
		Name:  "cost",
		Usage: "build cost report combining granular Evergreen and AWS data",
		Subcommands: []cli.Command{
			loadConfig(),
			generate(),
		},
	}
}

func loadConfig() cli.Command {
	return cli.Command{
		Name:  "load-config",
		Usage: "load a cost reporting configuration into the database from a file",
		Flags: dbFlags(
			cli.StringFlag{
				Name:  "file",
				Usage: "specify path to a build cost reporting config file",
			}),
		Action: func(c *cli.Context) error {
			env := sink.GetEnvironment()

			fileName := c.String("file")
			mongodbURI := c.String("dbUri")
			dbName := c.String("dbName")

			if err := configure(env, 2, true, mongodbURI, "", dbName); err != nil {
				return errors.WithStack(err)
			}

			conf, err := model.LoadCostConfig(fileName)
			if err != nil {
				return errors.WithStack(err)
			}

			if err = conf.Save(); err != nil {
				return errors.WithStack(err)
			}

			grip.Infoln("successfully saved cost configuration to database at:", mongodbURI)

			return nil
		},
	}
}

func collectLoop() cli.Command {
	return cli.Command{
		Name:  "collect",
		Usage: "collect a cost report every hour",
		Flags: dbFlags(costFlags()...),
		Action: func(c *cli.Context) error {
			mongodbURI := c.String("dbUri")
			dbName := c.String("dbName")
			env := sink.GetEnvironment()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := configure(env, 2, false, mongodbURI, "", dbName); err != nil {
				return errors.WithStack(err)
			}

			q, err := env.GetQueue()
			if err != nil {
				return errors.Wrap(err, "problem getting queue")
			}

			if err := q.Start(ctx); err != nil {
				return errors.Wrap(err, "problem starting queue")
			}

			amboy.IntervalQueueOperation(ctx, q, 30*time.Minute, time.Now(), true, func(queue amboy.Queue) error {
				lastHour := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, time.Local)

				id := fmt.Sprintf("brc-%s", lastHour)

				j := units.NewBuildCostReport(env, id)
				if err := queue.Put(j); err != nil {
					grip.Warning(err)
					return err
				}

				grip.Noticef("scheduled build cost report %s at [%s]", id, time.Now())
				return nil
			})

		},
	}
}

func generate() cli.Command {
	return cli.Command{
		Name:  "generate",
		Usage: "generate a report",
		Flags: costFlags(
			cli.StringFlag{
				Name:  "config",
				Usage: "path to configuration file, and EBS pricing information, is required",
			}),
		Action: func(c *cli.Context) error {
			start, err := time.Parse(sink.ShortDateFormat, c.String("start"))
			if err != nil {
				return errors.Wrapf(err, "problem parsing time from %s", c.String("start"))
			}
			file := c.String("config")
			dur := c.Duration("duration")

			conf, err := model.LoadCostConfig(file)
			if err != nil {
				return errors.Wrap(err, "problem loading cost configuration")
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := writeCostReport(ctx, conf, start, dur); err != nil {
				return errors.Wrap(err, "problem writing cost report")
			}

			return nil
		},
	}
}

func writeCostReport(ctx context.Context, conf *model.CostConfig, start time.Time, dur time.Duration) error {
	duration, err := conf.GetDuration(dur)
	if err != nil {
		return errors.Wrap(err, "Problem with duration")
	}

	report, err := cost.CreateReport(ctx, start, duration, conf)
	if err != nil {
		return errors.Wrap(err, "Problem generating report")
	}

	filename := fmt.Sprintf("%s_%s.json", report.Report.Begin, duration.String())
	if err := cost.Print(conf, report, filename); err != nil {
		return errors.Wrap(err, "Problem printing report")
	}

	return nil
}
