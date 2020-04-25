package operations

import (
	"context"
	"fmt"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/cost"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/units"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/amboy"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

const costReportDateFormat = "2006-01-02-15-04"

// Cost returns the entry point for the ./cedar spend sub-command,
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
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			fileName := c.String("file")
			mongodbURI := c.String(dbURIFlag)
			dbName := c.String(dbNameFlag)

			sc := newServiceConf(2, true, mongodbURI, "", dbName)
			sc.interactive = true

			if err := sc.setup(ctx); err != nil {
				return errors.WithStack(err)
			}

			env := cedar.GetEnvironment()

			conf := &model.CostConfig{}
			conf.Setup(env)

			if err := conf.Find(); err != nil {
				return errors.WithStack(err)
			}

			return errors.WithStack(utility.WriteJSONFile(fileName, conf))
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
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			fileName := c.String("file")
			mongodbURI := c.String(dbURIFlag)
			dbName := c.String(dbNameFlag)

			sc := newServiceConf(2, true, mongodbURI, "", dbName)
			sc.interactive = true

			if err := sc.setup(ctx); err != nil {
				return errors.WithStack(err)
			}

			env := cedar.GetEnvironment()

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
		Flags: mergeFlags(dbFlags(), costEvergreenOptionsFlags()),
		Action: func(c *cli.Context) error {
			mongodbURI := c.String(dbURIFlag)
			dbName := c.String(dbNameFlag)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sc := newServiceConf(2, true, mongodbURI, "", dbName)
			sc.interactive = true

			if err := sc.setup(ctx); err != nil {
				return errors.WithStack(err)
			}

			env := cedar.GetEnvironment()
			q := env.GetLocalQueue()
			if err := q.Start(ctx); err != nil {
				return errors.Wrap(err, "problem starting queue")
			}

			reports := &model.CostReports{}
			reports.Setup(env)

			opts := cost.EvergreenReportOptions{
				Duration:               time.Hour,
				DisableAll:             c.Bool(costDisableEVGAllFlag),
				DisableProjects:        c.Bool(costDisableEVGProjectsFlag),
				DisableDistros:         c.Bool(costDisableEVGDistrosFlag),
				AllowIncompleteResults: c.Bool(costContinueOnErrorFlag),
			}

			conf := amboy.QueueOperationConfig{
				ContinueOnError: true,
			}

			amboy.IntervalQueueOperation(ctx, q, 30*time.Minute, time.Now(), conf, func(ctx context.Context, queue amboy.Queue) error {
				now := time.Now().Add(-time.Hour).UTC()
				opts.StartAt = time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, time.UTC)

				id := fmt.Sprintf("bcr-%s", opts.StartAt.Format(costReportDateFormat))

				j := units.NewBuildCostReport(env, id, &opts)
				if err := queue.Put(ctx, j); err != nil {
					grip.Warning(err)
					return err
				}

				grip.Noticef("scheduled build cost report %s at [%s]", id, now)

				numReports, _ := reports.Count()
				grip.Info(message.Fields{
					"queue":         queue.Stats(ctx),
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
			mongodbURI := c.String(dbURIFlag)
			dbName := c.String(dbNameFlag)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sc := newServiceConf(2, true, mongodbURI, "", dbName)
			sc.interactive = true

			if err := sc.setup(ctx); err != nil {
				return errors.WithStack(err)
			}

			env := cedar.GetEnvironment()
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
		Flags: mergeFlags(costFlags(), costEvergreenOptionsFlags()),
		Action: func(c *cli.Context) error {
			start, err := time.Parse(cedar.ShortDateFormat, c.String(costStartFlag))
			if err != nil {
				return errors.Wrapf(err, "problem parsing time from %s", c.String(costStartFlag))
			}
			file := c.String(configFlag)
			dur := c.Duration(costDurationFlag)

			conf, err := model.LoadCostConfig(file)
			if err != nil {
				return errors.Wrapf(err, "problem loading cost configuration from %s", file)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			opts := cost.EvergreenReportOptions{
				StartAt:                start,
				Duration:               dur,
				DisableAll:             c.Bool(costDisableEVGAllFlag),
				DisableProjects:        c.Bool(costDisableEVGProjectsFlag),
				DisableDistros:         c.Bool(costDisableEVGDistrosFlag),
				AllowIncompleteResults: c.Bool(costContinueOnErrorFlag),
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
		Flags: mergeFlags(costFlags(), costEvergreenOptionsFlags()),
		Action: func(c *cli.Context) error {
			start, err := time.Parse(cedar.ShortDateFormat, c.String(costStartFlag))
			if err != nil {
				return errors.Wrapf(err, "problem parsing time from %s", c.String(costStartFlag))
			}
			file := c.String(configFlag)
			dur := c.Duration(costDurationFlag)

			conf, err := model.LoadCostConfig(file)
			if err != nil {
				return errors.Wrapf(err, "problem loading cost configuration from %s", file)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			opts := cost.EvergreenReportOptions{
				StartAt:                start,
				Duration:               dur,
				DisableAll:             c.Bool(costDisableEVGAllFlag),
				DisableProjects:        c.Bool(costDisableEVGProjectsFlag),
				DisableDistros:         c.Bool(costDisableEVGDistrosFlag),
				AllowIncompleteResults: c.Bool(costContinueOnErrorFlag),
			}

			report, err := cost.CreateReport(ctx, conf, &opts)
			if err != nil {
				return errors.Wrap(err, "Problem generating report")
			}
			report.Setup(cedar.GetEnvironment())
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
	report.Setup(cedar.GetEnvironment())

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
			mongodbURI := c.String(dbURIFlag)
			dbName := c.String(dbNameFlag)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sc := newServiceConf(2, true, mongodbURI, "", dbName)
			sc.interactive = true

			if err := sc.setup(ctx); err != nil {
				return errors.WithStack(err)
			}

			env := cedar.GetEnvironment()
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
