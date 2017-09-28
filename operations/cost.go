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
	"github.com/mongodb/grip/message"
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
			collectLoop(),
			write(),
			dump(),
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
			conf.Setup(env)

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
		Usage: "collect a cost report every hour, saving the results to mongodb",
		Flags: dbFlags(costEvergreenOptionsFlags()...),
		Action: func(c *cli.Context) error {
			mongodbURI := c.String("dbUri")
			dbName := c.String("dbName")
			env := sink.GetEnvironment()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := configure(env, 1, false, mongodbURI, "", dbName); err != nil {
				return errors.WithStack(err)
			}

			q, err := env.GetQueue()
			if err != nil {
				return errors.Wrap(err, "problem getting queue")
			}

			if err := q.Start(ctx); err != nil {
				return errors.Wrap(err, "problem starting queue")
			}

			reports := &model.CostReports{}
			reports.Setup(env)

			opts := cost.EvergreenReportOptions{
				Duration:               time.Hour,
				DisableAll:             c.BoolT("disableEvgAll"),
				DisableProjects:        c.BoolT("disableEvgProjects"),
				DisableDistros:         c.BoolT("disableEvgDistros"),
				AllowIncompleteResults: c.Bool("continueOnError"),
			}

			amboy.IntervalQueueOperation(ctx, q, 5*time.Minute, time.Now(), true, func(queue amboy.Queue) error {
				now := time.Now().Add(-time.Hour)
				opts.StartAt = time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, time.UTC)

				id := fmt.Sprintf("brc-%s", opts.StartAt)

				j := units.NewBuildCostReport(env, id, &opts)
				if err := queue.Put(j); err != nil {
					grip.Warning(err)
					return err
				}

				grip.Noticef("scheduled build cost report %s at [%s]", id, now)

				numReports, _ := reports.Count()
				grip.Info(message.Fields{
					"queue":         queue.Stats(),
					"reports-count": numReports,
					"scheduled":     id,
				})
				return nil
			})

			grip.Info("process blocking indefinitely to generate reports in the background")
			<-ctx.Done()

			grip.Alert("collection terminating")
			return errors.New("collection terminating")
		},
	}
}

func write() cli.Command {
	return cli.Command{
		Name:  "write",
		Usage: "collect and write a build cost report to a file.",
		Flags: costFlags(costEvergreenOptionsFlags()...),
		Action: func(c *cli.Context) error {
			start, err := time.Parse(sink.ShortDateFormat, c.String("start"))
			if err != nil {
				return errors.Wrapf(err, "problem parsing time from %s", c.String("start"))
			}
			file := c.String("config")
			dur := c.Duration("duration")

			conf, err := model.LoadCostConfig(file)
			if err != nil {
				return errors.Wrapf(err, "problem loading cost configuration from %s", file)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			opts := cost.EvergreenReportOptions{
				StartAt:                start,
				Duration:               dur,
				DisableAll:             c.BoolT("disableEvgAll"),
				DisableProjects:        c.BoolT("disableEvgProjects"),
				DisableDistros:         c.BoolT("disableEvgDistros"),
				AllowIncompleteResults: c.Bool("continueOnError"),
			}

			if err := writeCostReport(ctx, conf, &opts); err != nil {
				return errors.Wrap(err, "problem writing cost report")
			}

			return nil
		},
	}
}

func writeCostReport(ctx context.Context, conf *model.CostConfig, opts *cost.EvergreenReportOptions) error {
	duration, err := conf.GetDuration(opts.Duration)
	if err != nil {
		return errors.Wrap(err, "Problem with duration")
	}

	report, err := cost.CreateReport(ctx, conf, opts)
	if err != nil {
		return errors.Wrap(err, "Problem generating report")
	}

	fnDate := report.Report.Begin.Format("2006-01-02-15-04")

	filename := fmt.Sprintf("%s.%s.json", fnDate, duration)

	if err := cost.WriteToFile(conf, report, filename); err != nil {
		return errors.Wrap(err, "Problem printing report")
	}

	return nil
}

func dump() cli.Command {
	return cli.Command{
		Name:  "dump",
		Usage: "dump all cost reports to files",
		Flags: dbFlags(),
		Action: func(c *cli.Context) error {
			env := sink.GetEnvironment()

			mongodbURI := c.String("dbUri")
			dbName := c.String("dbName")

			if err := configure(env, 2, true, mongodbURI, "", dbName); err != nil {
				return errors.WithStack(err)
			}

			conf := &model.CostConfig{}
			conf.Setup(env)
			if err := conf.Find(); err != nil {
				return errors.Wrap(err, "problem loading cost config from the database")
			}

			reports := &model.CostReports{}
			reports.Setup(env)

			iter, err := reports.Iterator(time.Time{}, time.Now())
			if err != nil {
				return errors.WithStack(err)
			}
			defer iter.Close()

			report := &model.CostReport{}
			for iter.Next(report) {
				grip.Warning(cost.WriteToFile(conf, report, report.ID+".json"))
			}

			if err = iter.Err(); err != nil {
				return errors.Wrap(err, "problem querying for reports")
			}
			return nil
		},
	}

}
