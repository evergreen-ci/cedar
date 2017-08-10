package operations

import (
	"fmt"

	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/cost"
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
				Name: "duration",
			},
		},
		Action: func(c *cli.Context) error {
			start := c.String("start")
			duration := c.Duration("duration")
			var err error
			file := c.String("config")
			if file == "" {
				return errors.New("Configuration file is required")
			}
			err = configureSpend(file)
			if err != nil {
				return errors.Wrap(err, "Problem with config file")
			}
			config := sink.GetSpendConfig()
			if config.Pricing == nil {
				return errors.New("Configuration file requires EBS pricing information")
			}
			if !config.S3Info.IsValid() {
				return errors.New("Configuration file requires S3 bucket information")
			}
			if !config.EvergreenInfo.IsValid() {
				return errors.New("Configuration file requires evergreen user, key, and rootURL")
			}
			if duration == 0 { //empty duration
				duration, err = config.GetDuration()
				if err != nil {
					return errors.Wrap(err, "Problem with duration")
				}
			}
			ctx := context.Background()
			report, err := cost.CreateReport(ctx, start, duration, config)
			if err != nil {
				return errors.Wrap(err, "Problem generating report")
			}
			filename := fmt.Sprintf("%s_%s.txt", report.Report.Begin, duration.String())
			err = report.Print(config, filename)
			if err != nil {
				return errors.Wrap(err, "Problem printing report")
			}
			return nil
		},
	}
}
