package operations

import (
	"strings"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

////////////////////////////////////////////////////////////////////////
//
// Flag Name Constants

const (
	costStartFlag              = "start"
	costContinueOnErrorFlag    = "continue-on-error"
	costDurationFlag           = "duration"
	costDisableEVGAllFlag      = "disableEvgAll"
	costDisableEVGProjectsFlag = "disableEvgProjects"
	costDisableEVGDistrosFlag  = "disableEvgDistros"

	simpleLogIDFlag = "log"

	configFlag     = "config"
	pathFlagName   = "path"
	outputFlagName = "output"

	numWorkersFlag = "workers"
	bucketNameFlag = "bucket"

	dbURIFlag  = "dbUri"
	dbNameFlag = "dbName"

	clientHostFlag = "host"
	clientPortFlag = "port"

	flagNameflag = "flag"
)

////////////////////////////////////////////////////////////////////////
//
// Utility Functions

func joinFlagNames(ids ...string) string { return strings.Join(ids, ", ") }

func mergeFlags(in ...[]cli.Flag) []cli.Flag {
	out := []cli.Flag{}

	for idx := range in {
		out = append(out, in[idx]...)
	}

	return out
}

////////////////////////////////////////////////////////////////////////
//
// Flag Groups

func addPathFlag(flags ...cli.Flag) []cli.Flag {
	return append(flags, cli.StringFlag{
		Name:  joinFlagNames(pathFlagName, "filename", "file", "f"),
		Usage: "path to cedar input file",
	})
}

func addOutputPath(flags ...cli.Flag) []cli.Flag {
	return append(flags, cli.StringFlag{
		Name:  joinFlagNames(outputFlagName, "o"),
		Usage: "path to the output file",
		Value: "output.json",
	})
}

func depsFlags(flags ...cli.Flag) []cli.Flag {
	return append(flags, cli.StringFlag{
		Name:  joinFlagNames(pathFlagName, "filename", "file", "f"),
		Usage: "source path for dependency graph",
		Value: "deps.json",
	})
}

func simpleLogIDFlags(flags ...cli.Flag) []cli.Flag {
	return append(flags, cli.StringFlag{
		Name:  simpleLogIDFlag,
		Usage: "identifier for the log",
	})

}

func restServiceFlags(flags ...cli.Flag) []cli.Flag {
	return append(flags,
		cli.StringFlag{
			Name:  clientHostFlag,
			Usage: "host for the remote cedar instance.",
			Value: "http://localhost",
		},
		cli.IntFlag{
			Name:  clientPortFlag,
			Usage: "port for the remote cedar service. (Default port is 3000 if host is not explicitly set. If host is set, the port has no default.)",
		},
	)

}

func dbFlags(flags ...cli.Flag) []cli.Flag {
	return append(flags,
		cli.StringFlag{
			Name:   dbURIFlag,
			Usage:  "specify a mongodb connection string",
			Value:  "mongodb://localhost:27017",
			EnvVar: "CEDAR_MONGODB_URL",
		},
		cli.StringFlag{
			Name:   dbNameFlag,
			Usage:  "specify a database name to use",
			Value:  "cedar",
			EnvVar: "CEDAR_DATABASE_NAME",
		})
}

func addModifyFeatureFlagFlags(flags ...cli.Flag) []cli.Flag {
	return append(flags, cli.StringFlag{
		Name:  flagNameflag,
		Usage: "specify the name of the flag to set",
	})
}

func setFlagOrFirstPositional(name string) cli.BeforeFunc {
	return func(c *cli.Context) error {
		val := c.String(name)
		if val == "" {
			if c.NArg() != 1 {
				return errors.Errorf("must specify exactly one positional argument for '%s'", name)
			}

			val = c.Args().Get(0)
		}

		return c.Set(name, val)
	}
}

func baseFlags(flags ...cli.Flag) []cli.Flag {
	return append(flags,
		cli.IntFlag{
			Name:  numWorkersFlag,
			Usage: "specify the number of worker jobs this process will have",
			Value: 2,
		},
		cli.StringFlag{
			Name:   bucketNameFlag,
			Usage:  "specify a bucket name to use for storing data in s3",
			EnvVar: "CEDAR_BUCKET_NAME",
			Value:  "build-test-curator",
		})
}

func costFlags(flags ...cli.Flag) []cli.Flag {
	// get current time, round back to the start of the previous hour
	now := time.Now().Add(-time.Hour).UTC()

	defaultStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, time.UTC)

	return append(flags,
		cli.StringFlag{
			Name:  costStartFlag,
			Usage: "start time (UTC) in the format of YYYY-MM-DDTHH:MM",
			Value: defaultStart.Format(cedar.ShortDateFormat),
		},
		cli.BoolFlag{
			Name:  costContinueOnErrorFlag,
			Usage: "log but do not abort on collection errors",
		},
		cli.DurationFlag{
			Name:  costDurationFlag,
			Value: time.Hour,
		})
}

func costEvergreenOptionsFlags(flags ...cli.Flag) []cli.Flag {
	return append(flags,
		cli.StringFlag{
			Name:  configFlag,
			Usage: "path to configuration file, and EBS pricing information, is required",
		},
		cli.BoolFlag{
			Name:  costDisableEVGAllFlag,
			Usage: "specify to disable all evergreen data collection",
		},
		cli.BoolFlag{
			Name:  costDisableEVGProjectsFlag,
			Usage: "specify to disable all evergreen project data collection",
		},
		cli.BoolFlag{
			Name:  costDisableEVGDistrosFlag,
			Usage: "specify to disable all evergreen distro data collection",
		},
		cli.BoolFlag{
			Name:  costContinueOnErrorFlag,
			Usage: "specify to allow incomplete results",
		})

}
