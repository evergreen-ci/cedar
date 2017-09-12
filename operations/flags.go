package operations

import "github.com/urfave/cli"

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
