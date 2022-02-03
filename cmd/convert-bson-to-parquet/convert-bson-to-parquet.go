package main

import (
	"io"
	"log"
	"net/http"
	"os"

	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/utility"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"github.com/xitongsys/parquet-go-source/local"
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
			data, err := getBSON(c)
			if err != nil {
				return errors.Wrap(err, "getting BSON data")
			}

			var results testResultsDoc
			if err = bson.Unmarshal(data, &results); err != nil {
				return errors.Wrap(err, "unmarshalling test results")
			}

			fw, err := local.NewLocalFileWriter(c.String(outputFlag))
			if err != nil {
				return errors.Wrap(err, "creating local file writer")
			}
			defer fw.Close()

			pw, err := writer.NewParquetWriter(fw, new(model.ParquetTestResult), 4)
			if err != nil {
				return errors.Wrap(err, "creating new parquet writer")
			}

			for _, result := range results.Results {
				if err = pw.Write(result.ConvertToParquetTestResult()); err != nil {
					return errors.Wrap(err, "writing result")
				}
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

func mergeFlags(flags ...cli.Flag) []cli.Flag {
	mergedFlags := []cli.Flag{
		cli.StringFlag{
			Name:  s3URLFlag,
			Usage: "S3 download link for BSON file, must specify when not using a local file",
		},
		cli.StringFlag{
			Name:  localFileFlag,
			Usage: "local BSON file, must specify when not using an S3 download link",
		},
		cli.StringFlag{
			Name:     outputFlag,
			Usage:    "name of the output file, if none specified the output will go to stdout",
			Required: true,
		},
	}

	return append(mergedFlags, flags...)
}
