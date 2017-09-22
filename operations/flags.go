package operations

import (
	"time"

	"github.com/evergreen-ci/sink"
	"github.com/urfave/cli"
)

func dbFlags(flags ...cli.Flag) []cli.Flag {
	return append(flags,
		cli.StringFlag{
			Name:   "dbUri",
			Usage:  "specify a mongodb connection string",
			Value:  "mongodb://localhost:27017",
			EnvVar: "SINK_MONGODB_URL",
		},
		cli.StringFlag{
			Name:   "dbName",
			Usage:  "specify a database name to use",
			Value:  "sink",
			EnvVar: "SINK_DATABASE_NAME",
		})
}

func baseFlags(flags ...cli.Flag) []cli.Flag {
	return append(flags,
		cli.IntFlag{
			Name:  "workers",
			Usage: "specify the number of worker jobs this process will have",
			Value: 2,
		},
		cli.StringFlag{
			Name:   "bucket",
			Usage:  "specify a bucket name to use for storing data in s3",
			EnvVar: "SINK_BUCKET_NAME",
			Value:  "build-test-curator",
		})
}

func costFlags(flags ...cli.Flag) []cli.Flag {
	// get current time, round back to the start of the previous hour
	now := time.Now().Add(-time.Hour)
	defaultStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, time.UTC)

	return append(flags,
		cli.StringFlag{
			Name:  "start",
			Usage: "start time (UTC) in the format of YYYY-MM-DDTHH:MM",
			Value: defaultStart.Format(sink.ShortDateFormat),
		},
		cli.DurationFlag{
			Name:  "duration",
			Value: time.Hour,
		})
}

func depsFlags(flags ...cli.Flag) []cli.Flag {
	return append(flags,
		cli.StringFlag{
			Name:  "path",
			Usage: "source path for dependency graph",
			Value: "deps.json",
		})
}
