package operations

import (
	"fmt"
	"time"

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
			file := c.String("config")
			dur := c.Duration("duration")
			env := sink.GetEnvironment()

			conf, err := loadCostConfig(env, file)
			if err != nil {
				return errors.Wrap(err, "problem loading cost configuration")
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := writeCostReport(ctx, conf.Cost, start, dur); err != nil {
				return errors.Wrap(err, "problem writing cost report")
			}

			return nil
		},
	}
}

func loadCostConfig(env sink.Environment, file string) (*sink.Configuration, error) {
	if file == "" {
		return nil, errors.New("Configuration file is required")
	}

	costConf, err := cost.LoadConfig(file)
	if err != nil {
		return nil, errors.Wrap(err, "problem loading config")
	}

	conf, err := env.GetConf()
	if err != nil {
		return nil, errors.Wrap(err, "problem getting application configuration")
	}

	conf.Cost = costConf
	if err = env.SetConf(conf); err != nil {
		return nil, errors.Wrap(err, "problem saving config")
	}

	return conf, nil
}

func writeCostReport(ctx context.Context, conf *cost.Config, start string, dur time.Duration) error {
	duration, err := conf.GetDuration(dur)
	if err != nil {
		return errors.Wrap(err, "Problem with duration")
	}

	report, err := cost.CreateReport(ctx, start, duration, conf)
	if err != nil {
		return errors.Wrap(err, "Problem generating report")
	}

	filename := fmt.Sprintf("%s_%s.txt", report.Report.Begin, duration.String())
	if err := report.Print(conf, filename); err != nil {
		return errors.Wrap(err, "Problem printing report")
	}

	return nil
}
