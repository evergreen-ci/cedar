package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/pail"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"github.com/xitongsys/parquet-go/writer"
)

const (
	projectsFlagName   = "projects"
	startDateFlagName  = "start_date"
	endDateFlagName    = "end_date"
	destBucketFlagName = "dest_bucket"
	prefixFlagName     = "prefix"
	profileFlagName    = "profile"
	mongoURIFlagName   = "mongo_uri"

	dateFormat = "2006-01-02"
)

func main() {
	app := cli.NewApp()
	app.Name = "convert-bson-to-parquet"
	app.Usage = "convert BSON data to Parquet"
	app.Commands = []cli.Command{
		convertTestResults(),
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func convertTestResults() cli.Command {
	return cli.Command{
		Name:  "test-results",
		Usage: "convert BSON test results to Parquet",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     projectsFlagName,
				Usage:    "list of comma separated projects to fetch",
				Required: true,
			},
			&cli.StringFlag{
				Name:     startDateFlagName,
				Usage:    "the start date (inclusive) of the targeted interval, format YYYY-MM-DD",
				Required: true,
			},
			&cli.StringFlag{
				Name:     endDateFlagName,
				Usage:    "the end date (inclusive) of the targeted interval, format YYYY-MM-DD",
				Required: true,
			},
			&cli.StringFlag{
				Name:     destBucketFlagName,
				Usage:    "the name of the destination S3 bucket",
				Required: true,
			},
			&cli.StringFlag{
				Name:  prefixFlagName,
				Usage: "destination bucket prefix",
			},
			&cli.StringFlag{
				Name:  profileFlagName,
				Usage: "profile of AWS credentials, if not default",
			},
			&cli.StringFlag{
				Name:     mongoURIFlagName,
				Usage:    "connection string for the Cedar database",
				Required: true,
			},
		},
		Action: func(c *cli.Context) error {
			startAt, err := time.Parse(dateFormat, c.String(startDateFlagName))
			if err != nil {
				return errors.New("invalid start_at")
			}
			endAt, err := time.Parse(dateFormat, c.String(endDateFlagName))
			if err != nil {
				return errors.New("invalid end_at")
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			env, err := setupEnv(ctx, c)
			if err != nil {
				return errors.Wrap(err, "setting up environment")
			}

			testResults, err := model.FindTestResultsByProject(ctx, env, model.FindTestResultsByProjectOptions{
				Projects: strings.Split(c.String(projectsFlagName), ","),
				StartAt:  startAt,
				EndAt:    endAt,
			})
			if err != nil {
				return errors.Wrap(err, "getting test results from the database")
			}

			b, err := pail.NewS3Bucket(pail.S3Options{
				Name:                     c.String(destBucketFlagName),
				Prefix:                   c.String(prefixFlagName),
				Region:                   "us-east-1",
				Permissions:              pail.S3PermissionsPrivate,
				SharedCredentialsProfile: c.String(profileFlagName),
				MaxRetries:               10,
			})
			if err != nil {
				return errors.Wrap(err, "creating destination bucket")
			}

			for _, tr := range testResults {
				ptr, err := tr.DownloadAndConvertToParquet(ctx)
				if err != nil {
					return errors.Wrap(err, "downloading and converting test result")
				}

				w, err := b.Writer(ctx, tr.PrestoPartitionKey())
				if err != nil {
					return errors.Wrap(err, "creating bucket writer")
				}
				pw, err := writer.NewParquetWriterFromWriter(w, new(model.ParquetTestResults), 4)
				if err != nil {
					return errors.Wrap(err, "creating new parquet writer")
				}
				if err = pw.Write(ptr); err != nil {
					return errors.Wrap(err, "writing results to parquet")
				}
				if err = pw.WriteStop(); err != nil {
					return errors.Wrap(err, "stopping parquet writer")
				}
				if err := w.Close(); err != nil {
					return errors.Wrap(err, "writing parquet to bucket")
				}
			}

			return nil
		},
	}
}

func setupEnv(ctx context.Context, c *cli.Context) (cedar.Environment, error) {
	env, err := cedar.NewEnvironment(ctx, "convert-bson-to-parquet", &cedar.Configuration{
		MongoDBURI:         c.String(mongoURIFlagName),
		DatabaseName:       "cedar",
		SocketTimeout:      time.Minute,
		NumWorkers:         2,
		DisableRemoteQueue: true,
		DisableCache:       true,
	})
	if err != nil {
		return nil, err
	}

	return env, nil
}
