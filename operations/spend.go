package operations

import (
	"fmt"
	"time"

	"github.com/evergreen-ci/sink/cost"
	"github.com/evergreen-ci/sink/model"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
)

//Spend returns the entry point for the ./sink spend sub-command,
//which has required flags.
func Spend() cli.Command {
	// get current time, round back to the start of the previous hour
	return cli.Command{
		Name:  "spend",
		Usage: "generate a report covering Evergreen and AWS data",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "config",
				Usage: "path to configuration file, and EBS pricing information, is required",
			},
			cli.StringFlag{
				Name:  "start",
				Usage: "start time (UTC) in the format of YYYY-MM-DDTHH:MM",
			},
			cli.DurationFlag{
				Name:  "duration",
				Value: time.Hour,
			},
		},
		Action: func(c *cli.Context) error {
			start, err := time.Parse("2006-01-02T15:04", c.String("start"))
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
