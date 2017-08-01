package operations

import (
	"fmt"

	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/cost"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
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
				Name: "granularity",
			},
		},
		Action: func(c *cli.Context) error {
			start := c.String("start")
			granularity := c.Duration("granularity")
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
			if granularity == 0 { //empty duration
				granularity, err = config.GetGranularity()
				if err != nil {
					return errors.Wrap(err, "Problem with granularity")
				}
			}

			report, err := cost.CreateReport(start, granularity, config)
			if err != nil {
				return errors.Wrap(err, "Problem generating report")
			}
			filename := fmt.Sprintf("%s_%s.txt", report.Report.Begin, granularity.String())
			err = report.Print(config, filename)
			if err != nil {
				return errors.Wrap(err, "Problem printing report")
			}
			return nil
		},
	}
}
