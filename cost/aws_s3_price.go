package cost

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/evergreen-ci/cedar/util"
	"github.com/pkg/errors"
)

const (
	// These index constants represent the column numbers in the billing report for the
	// account owner column, service name column, and cost column (the last column).
	ownerIdx    = 9
	serviceIdx  = 12
	costIdx     = 28
	contentType = "text/csv"
)

// S3Info stores information necessary to call the API
// Note that KeyStart will have "-YYYY-MM.csv" appended to it
type S3Info struct {
	KeyStart string `yaml:"s3key_start"`
	Bucket   string `yaml:"s3bucket"`
	Owner    string
}

// For now, this function returns an example CSV.
func (c *AWSClient) fetchS3Spending(info *S3Info, reportRange util.TimeRange) ([][]string, error) {
	year, monthNum, _ := reportRange.StartAt.Date()
	month := fmt.Sprintf("%d", monthNum)
	if monthNum < 10 {
		month = fmt.Sprintf("0%d", monthNum)
	}
	key := fmt.Sprintf("%s-%d-%s.csv", info.KeyStart, year, month)

	responseType := contentType
	input := &s3.GetObjectInput{
		Bucket:              &info.Bucket,
		Key:                 &key,
		ResponseContentType: &responseType,
	}
	res, err := c.s3Client.GetObject(input)
	if err != nil {
		return [][]string{}, err
	}
	defer res.Body.Close()

	csvReader := csv.NewReader(res.Body)
	body, err := csvReader.ReadAll()
	if err != nil {
		return [][]string{}, errors.Wrap(err, "Error reading from file")
	}
	return body, nil
}

// GetS3Cost retrieves the current AWS CSV file and returns the S3 cost.
func (c *AWSClient) GetS3Cost(info *S3Info, reportRange util.TimeRange) (float32, error) {
	lines, err := c.fetchS3Spending(info, reportRange)
	if err != nil {
		return 0.0, err
	}
	var price float32
	for i, line := range lines {
		if i == 0 || len(line) < costIdx {
			// line is a header, or line is missing information
			continue
		}
		lineName := strings.ToLower(line[ownerIdx])
		serviceName := line[serviceIdx]
		if lineName == strings.ToLower(info.Owner) && strings.Contains(serviceName, "S3") {
			linePrice, err := strconv.ParseFloat(line[costIdx], 32)
			if err != nil {
				return 0.0, err
			}
			price += float32(linePrice)
		}
	}
	return price, nil
}
