package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/pail"
	"github.com/evergreen-ci/utility"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/types"
	"github.com/xitongsys/parquet-go/writer"
	"go.mongodb.org/mongo-driver/bson"
)

const (
	s3URLFlag     = "s3"
	localFileFlag = "local"
	outputFlag    = "out"
)

func main() {
	app := cli.NewApp()
	app.Name = "convert-bson-to-parquet"
	app.Usage = "convert a specified BSON file to an equivalent Parquet file"
	app.Commands = []cli.Command{
		convertTestResults(),
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

type testResultsDoc struct {
	Results []model.TestResult `bson:"results"`
}

func convertTestResults() cli.Command {
	return cli.Command{
		Name:  "test-results",
		Usage: "convert a test results BSON file to Parquet",
		Flags: mergeFlags(),
		Action: func(c *cli.Context) error {
			if fn := c.String("csv-file"); fn != "" {
				return convertAndUploadFromCSV(fn)
			}

			data, err := getBSON(c)
			if err != nil {
				return errors.Wrap(err, "getting BSON data")
			}

			var results testResultsDoc
			if err = bson.Unmarshal(data, &results); err != nil {
				return errors.Wrap(err, "unmarshalling test results")
			}
			if len(results.Results) == 0 {
				return nil
			}

			created := types.TimeToTIMESTAMP_MILLIS(results.Results[0].TaskCreateTime.UTC(), true)
			exec := int32(results.Results[0].Execution)
			parquetResults := model.ParquetTestResults{
				Version:        utility.ToStringPtr("some-version"),
				Variant:        utility.ToStringPtr("some-variant"),
				TaskID:         &results.Results[0].TaskID,
				Execution:      &exec,
				TaskCreateTime: &created,
				Results:        make([]model.ParquetTestResult, 0),
			}
			for _, result := range results.Results {
				converted := result.ConvertToParquetTestResult()
				parquetResults.Results = append(parquetResults.Results, converted)
			}

			fw, err := local.NewLocalFileWriter(c.String(outputFlag))
			if err != nil {
				return errors.Wrap(err, "creating local file writer")
			}
			defer fw.Close()

			pw, err := writer.NewParquetWriter(fw, new(model.ParquetTestResults), 4)
			if err != nil {
				return errors.Wrap(err, "creating new parquet writer")
			}

			if err = pw.Write(parquetResults); err != nil {
				return errors.Wrap(err, "writing result")
			}
			if err = pw.WriteStop(); err != nil {
				return errors.Wrap(err, "stopping parquet writer")
			}

			return nil
		},
	}
}

func getBSON(c *cli.Context) ([]byte, error) {
	url := c.String(s3URLFlag)
	filepath := c.String(localFileFlag)
	if url == "" && filepath == "" {
		return nil, errors.New("must specify either an S3 URL or local filepath")
	}
	if url != "" && filepath != "" {
		return nil, errors.New("cannot specify both an S3 URL and a local filepath")
	}

	if url != "" {
		data, err := downloadFile(url)
		return data, errors.Wrap(err, "downloading S3 file")
	}

	return os.ReadFile(filepath)
}

func downloadFile(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "creating request")
	}

	client := utility.GetHTTPClient()
	defer utility.PutHTTPClient(client)
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "requesting file")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("request failed with with status: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	return data, errors.Wrap(err, "reading response body")
}

type metadata struct {
	Project string
	Variant string
	Version string
	Prefix  string
}

func readCSV(filename string) ([]metadata, error) {
	fr, err := os.Open(filename)
	if err != nil {
		return nil, errors.Wrap(err, "opening csv file")
	}
	defer fr.Close()

	r := csv.NewReader(fr)
	rows, err := r.ReadAll()
	if err != nil {
		return nil, errors.Wrap(err, "reading csv")
	}

	data := make([]metadata, len(rows)-1)
	for i := 1; i < len(rows); i++ {
		data[i-1] = metadata{
			Prefix:  rows[i][1],
			Project: rows[i][2],
			Version: rows[i][3],
			Variant: rows[i][4],
		}
	}

	return data, nil
}

func convertAndUploadFromCSV(filename string) error {
	md, err := readCSV(filename)
	if err != nil {
		return err
	}

	opts := pail.S3Options{
		Name:                     "mongodatalake-dev-prod-staging",
		Prefix:                   "cedar_test_results_landing",
		Region:                   "us-east-1",
		Permissions:              pail.S3PermissionsPrivate,
		SharedCredentialsProfile: "presto",
		MaxRetries:               10,
	}
	b, err := pail.NewS3Bucket(opts)
	if err != nil {
		return errors.Wrap(err, "creating bucket")
	}

	for _, data := range md {
		results, err := getBSONFromPrefix(data.Prefix)
		if err != nil {
			return errors.Wrap(err, "getting BSON test result. Prefix="+data.Prefix)
		}
		if len(results.Results) == 0 {
			continue
		}

		created := types.TimeToTIMESTAMP_MILLIS(results.Results[0].TaskCreateTime.UTC(), true)
		parquetResults := model.ParquetTestResults{
			Version:        &data.Version,
			Variant:        &data.Variant,
			TaskID:         &results.Results[0].TaskID,
			Execution:      utility.ToInt32Ptr(int32(results.Results[0].Execution)),
			TaskCreateTime: &created,
			Results:        make([]model.ParquetTestResult, 0),
		}
		for _, result := range results.Results {
			converted := result.ConvertToParquetTestResult()
			parquetResults.Results = append(parquetResults.Results, converted)
		}

		key := fmt.Sprintf("project=%s/task_create_iso=%s", data.Project, results.Results[0].TaskCreateTime.UTC().Format("2021-01-01"))
		w, err := b.Writer(context.Background(), key)
		if err != nil {
			return errors.Wrap(err, "creating bucket writer")
		}

		pw, err := writer.NewParquetWriterFromWriter(w, new(model.ParquetTestResults), 4)
		if err != nil {
			return errors.Wrap(err, "creating new parquet writer")
		}

		if err = pw.Write(parquetResults); err != nil {
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
}

func getBSONFromPrefix(prefix string) (*testResultsDoc, error) {
	opts := pail.S3Options{
		Name: "cedar-test-results",
		// Prefix:                   prefix,
		Region:                   "us-east-1",
		Permissions:              pail.S3PermissionsPublicRead,
		SharedCredentialsProfile: "cedar",
		MaxRetries:               10,
	}
	b, err := pail.NewS3Bucket(opts)
	if err != nil {
		return nil, errors.Wrap(err, "creating bucket")
	}

	r, err := b.Get(context.Background(), prefix+"/test_results")
	if err != nil {
		return nil, errors.Wrap(err, "getting test results")
	}
	defer r.Close()

	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, errors.Wrap(err, "reading test results")
	}

	var results testResultsDoc
	if err = bson.Unmarshal(data, &results); err != nil {
		return nil, errors.Wrap(err, "unmarshalling test results")
	}

	return &results, nil
}

func mergeFlags(flags ...cli.Flag) []cli.Flag {
	mergedFlags := []cli.Flag{
		&cli.StringFlag{
			Name:  s3URLFlag,
			Usage: "S3 download link for BSON file, must specify when not using a local file",
		},
		&cli.StringFlag{
			Name:  localFileFlag,
			Usage: "local BSON file, must specify when not using an S3 download link",
		},
		&cli.StringFlag{
			Name: "csv-file",
		},
		&cli.StringFlag{
			Name:     outputFlag,
			Usage:    "name of the output file, if none specified the output will go to stdout",
			Required: true,
		},
	}

	return append(mergedFlags, flags...)
}
