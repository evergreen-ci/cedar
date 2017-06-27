package operations

import (
	"time"

	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/cost"
	"github.com/mongodb/grip"
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
				Name:  "start",
				Usage: "start time (UTC) in the format of YYYY-MM-DDTHH:MM",
			},
			cli.DurationFlag{
				Name:  "granularity",
				Value: 4 * time.Hour, //Default value
			},
			cli.StringFlag{
				Name:  "config",
				Usage: "path to configuration file",
			},
		},
		Action: func(c *cli.Context) error {
			var startTime time.Time
			start := c.String("start")
			granularity := c.Duration("granularity")
			var err error
			file := c.String("config")
			if file != "" {
				err = configureSpend(file)
				if err != nil {
					return err
				}
			}

			if granularity == 0 { //empty duration
				granularity, err = sink.GetSpendConfig().GetGranularity()
				if err != nil {
					return err
				}
			}
			startTime, err = cost.GetStartTime(start, granularity)
			if err != nil {
				return err
			}

			grip.Noticef("Not yet implemented: will generate a report for the "+
				"given %s and %s", startTime, granularity)
			return nil
		},
	}
}
