package operations

import (
	"fmt"
	"time"

	"context"

	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/cost"
	"github.com/evergreen-ci/sink/model"
	"github.com/evergreen-ci/sink/units"
	"github.com/mongodb/amboy"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

const costReportDateFormat = "2006-01-02-15-04"

// Cost returns the entry point for the ./sink spend sub-command,
// which has required flags.
func Cost() cli.Command {
	return cli.Command{
		Name:  "cost",
		Usage: "build cost report combining granular Evergreen and AWS data",
		Subcommands: []cli.Command{
			loadCostConfig(),
			dumpCostConfig(),
			printScrn(),
			collectLoop(),
			write(),
			dump(),
			summarize(),
		},
	}
}

func dumpCostConfig() cli.Command {
	return cli.Command{
		Name:  "dump-config",
		Usage: "download a cost reporting configuration from the database into a file",
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

			conf := &model.CostConfig{}
			conf.Setup(env)

			if err := conf.Find(); err != nil {
				return errors.WithStack(err)
			}

			return errors.WithStack(writeJSON(fileName, conf))
		},
	}
}

func loadCostConfig() cli.Command {
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
				DisableAll:             c.Bool("disableEvgAll"),
				DisableProjects:        c.Bool("disableEvgProjects"),
				DisableDistros:         c.Bool("disableEvgDistros"),
				AllowIncompleteResults: c.Bool("continueOnError"),
			}

			conf := amboy.QueueOperationConfig{
				ContinueOnError: true,
			}

			amboy.IntervalQueueOperation(ctx, q, 30*time.Minute, time.Now(), conf, func(queue amboy.Queue) error {
				now := time.Now().Add(-time.Hour).UTC()
				opts.StartAt = time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, time.UTC)

				id := fmt.Sprintf("bcr-%s", opts.StartAt.Format(costReportDateFormat))

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

func summarize() cli.Command {
	return cli.Command{
		Name:  "summarize",
		Usage: "reads reports from the database and writes summaries",
		Flags: dbFlags(),
		Action: func(c *cli.Context) error {
			mongodbURI := c.String("dbUri")
			dbName := c.String("dbName")
			env := sink.GetEnvironment()
			if err := configure(env, 1, false, mongodbURI, "", dbName); err != nil {
				return errors.WithStack(err)
			}

			reports := &model.CostReports{}
			reports.Setup(env)
			iter, err := reports.Iterator(time.Time{}, time.Now())
			if err != nil {
				return errors.WithStack(err)
			}

			num, err := reports.Count()
			if err != nil {
				return errors.WithStack(err)
			}
			grip.Infof("found %d reports", num)

			report := &model.CostReport{}
			catcher := grip.NewBasicCatcher()
			count := 0
			for iter.Next(report) {
				count++
				report.Setup(env)
				summary := model.NewCostReportSummary(report)
				grip.Info(message.Fields{
					"summary": summary,
					"number":  count,
					"total":   num,
				})
				catcher.Add(summary.Save())
			}
			if err = iter.Close(); err != nil {
				return errors.WithStack(err)
			}

			return catcher.Resolve()
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
				DisableAll:             c.Bool("disableEvgAll"),
				DisableProjects:        c.Bool("disableEvgProjects"),
				DisableDistros:         c.Bool("disableEvgDistros"),
				AllowIncompleteResults: c.Bool("continueOnError"),
			}

			if err := writeCostReport(ctx, conf, &opts); err != nil {
				return errors.Wrap(err, "problem writing cost report")
			}

			return nil
		},
	}
}

func printScrn() cli.Command {
	return cli.Command{
		Name:  "print",
		Usage: "print a cost report to the terminal",
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
				DisableAll:             c.Bool("disableEvgAll"),
				DisableProjects:        c.Bool("disableEvgProjects"),
				DisableDistros:         c.Bool("disableEvgDistros"),
				AllowIncompleteResults: c.Bool("continueOnError"),
			}

			report, err := cost.CreateReport(ctx, conf, &opts)
			if err != nil {
				return errors.Wrap(err, "Problem generating report")
			}
			report.Setup(sink.GetEnvironment())
			fmt.Println(report.String())
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
	report.Setup(sink.GetEnvironment())

	filename := getCostReportFn(report.Report.Range.StartAt, duration)

	if err := cost.WriteToFile(conf, report, filename); err != nil {
		return errors.Wrap(err, "Problem printing report")
	}

	return nil
}

func getCostReportFn(startAt time.Time, duration time.Duration) string {
	return fmt.Sprintf("%s.%s.json", startAt.Format(costReportDateFormat), duration)
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

			duration, err := conf.GetDuration(time.Hour)
			if err != nil {
				return errors.Wrap(err, "Problem with duration")
			}

			reports := &model.CostReports{}
			reports.Setup(env)

			iter, err := reports.Iterator(time.Time{}, time.Now().UTC())
			if err != nil {
				return errors.WithStack(err)
			}
			defer iter.Close()

			report := &model.CostReport{}
			for iter.Next(report) {
				fn := getCostReportFn(report.Report.Range.StartAt, duration)
				grip.Warning(cost.WriteToFile(conf, report, fn))
			}

			if err = iter.Err(); err != nil {
				return errors.Wrap(err, "problem querying for reports")
			}
			return nil
		},
	}

}
