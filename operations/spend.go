package operations

import (
	"time"

	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

//Spend returns the entry point for the ./sink spend sub-command,
//which has required flags.
func Spend() cli.Command {
	// get current time, round back to the start of the previous hour
	const layout = "2017-05-23T17:00"
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
		},
		Action: func(c *cli.Context) error {
			var startTime time.Time
			var err error
			start := c.String("start")
			granularity := c.Duration("granularity")

			if start != "" {
				startTime, err = time.Parse(layout, start)
				if err != nil {
					return errors.Wrap(err, "incorrect start format: "+
						"should be YYYY-MM-DDTHH:MM")
				}
			} else {
				now := time.Now()
				startTime = now.Add(-granularity)
			}
			grip.Noticef("Not yet implemented: will generate a report for the "+
				"given %s and %s", startTime, granularity)
			return nil
		},
	}

}
