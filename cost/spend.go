package cost

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/evergreen-ci/sink/amazon"
	"github.com/evergreen-ci/sink/evergreen"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

const (
	layout = "2006-01-02T15:04" //Using reference Mon Jan 2 15:04:05 -0700 MST 2006
	aws    = "aws"
)

type timeRange struct {
	start time.Time
	end   time.Time
}

// GetDuration returns the duration in the config file as type time.Duration.
// If the config file duration is empty, we return the default.
func (c *Config) GetDuration(input time.Duration) (time.Duration, error) {
	if input < time.Minute {
		grip.Warningf("input time is %s, falling back to the config file or default", input)
		input = time.Duration(0)
	}

	configDur := c.Opts.Duration
	var err error
	duration := time.Hour //default value
	if configDur != "" {
		duration, err = time.ParseDuration(configDur)
		if err != nil {
			return 0, errors.Wrapf(err, "Could not parse duration %s", configDur)
		}
	}
	return duration, nil
}

// UpdateSpendProviders updates the given config file's providers to include
// the providers in the given Provider array. If the provider already exists,
// we update the cost.
func (c *Config) UpdateSpendProviders(newProv []Provider) {
	for _, new := range newProv {
		added := false
		for idx := range c.Providers {
			if new.Name == c.Providers[idx].Name {
				c.Providers[idx].Cost = new.Cost

				added = true
				break
			}
		}
		if !added {
			c.Providers = append(c.Providers, new)
		}
	}
}

// getTimes takes in a string of the form "YYYY-MM-DDTHH:MM" as the start
// time for the report, and converts this to time.Time type. If the given string
// is empty, we instead default to using the current time minus the duration.
func getTimes(s string, duration time.Duration) (timeRange, error) {
	var startTime, endTime time.Time
	var err error
	var res timeRange
	if s != "" {
		startTime, err = time.Parse(layout, s)
		if err != nil {
			return res, errors.Wrap(err, "incorrect start format: "+
				"should be YYYY-MM-DDTHH:MM")
		}
		endTime = startTime.Add(duration)
	} else {
		endTime = time.Now()
		startTime = endTime.Add(-duration)
	}
	res.start = startTime
	res.end = endTime

	return res, nil
}

// roundUp rounds the input number up, with places representing the number of decimal places.
func roundUp(input float64, places int) float64 {
	var round float64
	pow := math.Pow(10, float64(places))
	digit := pow * input
	round = math.Ceil(digit)
	newVal := round / pow
	return newVal
}

// avg returns the average of the vals
func avg(vals []float64) float64 {
	total := 0.0
	for _, v := range vals {
		total += v
	}
	avg := total / float64(len(vals))
	return roundUp(avg, 2)
}

// setItems sets the number of launched and terminated instances of the given cost item.
// The sums are calculated from the information in the Item array.
func (res *Item) setSums(items []*amazon.Item) {
	res.Launched, res.Terminated, res.TotalHours = 0, 0, 0
	for _, item := range items {
		if item.Launched {
			if item.Count != 0 {
				res.Launched += item.Count
			} else {
				res.Launched++
			}
		}
		if item.Terminated {
			if item.Count != 0 {
				res.Terminated += item.Count
			} else {
				res.Terminated++
			}
		}
		res.TotalHours += item.Uptime
	}
}

// avgItems sets the average price, fixed price, and uptime of the given cost item.
// The averages are calculated from the information in the Item array.
func (res *Item) setAverages(items []*amazon.Item) {
	var prices, uptimes, fixedPrices []float64
	for _, item := range items {
		if item.Price != 0.0 {
			prices = append(prices, item.Price)
		}
		if item.FixedPrice != 0.0 {
			fixedPrices = append(fixedPrices, item.FixedPrice)
		}
		if item.Uptime != 0 {
			uptimes = append(uptimes, float64(item.Uptime))
		}
	}
	if len(prices) != 0 {
		res.AvgPrice = float32(avg(prices))
	}
	if len(fixedPrices) != 0 {
		res.FixedPrice = float32(avg(fixedPrices))
	}
	if len(uptimes) != 0 {
		res.AvgUptime = float32(avg(uptimes))
	}
}

// createItemFromEC2Instance creates a new cost.Item using a key/item array pair.
func createCostItemFromAmazonItems(key amazon.ItemKey, items []*amazon.Item) *Item {
	item := &Item{
		Name:     key.Name,
		ItemType: string(key.ItemType),
	}
	item.setSums(items)
	item.setAverages(items)

	return item
}

//getAWSAccountByOwner gets account information using the API keys labeled by the owner string.
func getAWSAccountByOwner(ctx context.Context, reportRange amazon.TimeRange, config *Config,
	owner string) (*Account, error) {
	grip.Infof("Compiling data for account owner %s", owner)
	client, err := amazon.NewClient(owner)
	if err != nil {
		return nil, errors.Wrapf(err, "Problem getting client %s", owner)
	}
	instances, err := client.GetEC2Instances(ctx, reportRange)
	if err != nil {
		return nil, errors.Wrap(err, "Problem getting EC2 instances")
	}
	instances, err = client.AddEBSItems(ctx, instances, reportRange, &config.Amazon.EBSPrices)
	if err != nil {
		return nil, errors.Wrap(err, "Problem getting EBS instances")
	}
	s3info := config.Amazon.S3Info
	s3info.Owner = owner
	ec2Service := &Service{
		Name: string(amazon.EC2Service),
	}
	ebsService := &Service{
		Name: string(amazon.EBSService),
	}
	s3Service := &Service{
		Name: string(amazon.S3Service),
	}
	s3Service.Cost, err = client.GetS3Cost(&s3info, reportRange)
	if err != nil {
		return nil, errors.Wrap(err, "Error fetching S3 Spending CSV")
	}
	grip.Infof("Iterating through %d instance types", len(instances))
	for key, items := range instances {
		item := createCostItemFromAmazonItems(key, items)
		if key.Service == amazon.EC2Service {
			ec2Service.Items = append(ec2Service.Items, item)
		} else {
			ebsService.Items = append(ebsService.Items, item)
		}
	}
	account := &Account{
		Name:     owner,
		Services: []*Service{ec2Service, ebsService, s3Service},
	}
	return account, nil
}

// getAWSAccounts takes in a range for the report, and returns an array of accounts
// containing EC2 and EBS instances.
func getAWSAccounts(ctx context.Context, reportRange timeRange, config *Config) ([]*Account, error) {
	awsReportRange := amazon.TimeRange{
		Start: reportRange.start,
		End:   reportRange.end,
	}
	var allAccounts []*Account
	for _, owner := range config.Amazon.Accounts {
		account, err := getAWSAccountByOwner(ctx, awsReportRange, config, owner)
		if err != nil {
			return nil, err
		}
		allAccounts = append(allAccounts, account)
	}
	return allAccounts, nil
}

// getAWSProvider specifically creates a provider for AWS and populates those accounts
func getAWSProvider(ctx context.Context, reportRange timeRange, config *Config) (*Provider, error) {
	var err error
	res := &Provider{
		Name: aws,
	}
	res.Accounts, err = getAWSAccounts(ctx, reportRange, config)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// getAllProviders returns the AWS provider and any providers in the config file
func getAllProviders(ctx context.Context, reportRange timeRange, config *Config) ([]Provider, error) {
	awsProvider, err := getAWSProvider(ctx, reportRange, config)
	if err != nil {
		return nil, err
	}

	providers := []Provider{*awsProvider}

	providers = append(providers, config.Providers...)

	return providers, nil
}

// CreateReport returns an Output using a start string, duration, and Config information.
func CreateReport(ctx context.Context, start string, duration time.Duration, config *Config) (*Output, error) {
	grip.Info("Creating the report\n")
	output := &Output{}
	reportRange, err := getTimes(start, duration)
	if err != nil {
		return output, errors.Wrap(err, "Problem retrieving report start and end")
	}

	output.Providers, err = getAllProviders(ctx, reportRange, config)
	if err != nil {
		return output, errors.Wrap(err, "Problem retrieving providers information")
	}

	c := evergreen.NewClient(&http.Client{}, &config.Evergreen)
	evg, err := getEvergreenData(c, reportRange.start, duration)
	if err != nil {
		return &Output{}, errors.Wrap(err, "Problem retrieving evergreen information")
	}
	output.Evergreen = *evg

	output.Report = Report{
		Begin:     reportRange.start.String(),
		End:       reportRange.end.String(),
		Generated: time.Now().String(),
	}
	return output, nil
}

// Print writes the report to the given file, using the directory in the config file.
// If no directory is given, print report to stdout.
func (report *Output) Print(config *Config, filepath string) error {
	jsonReport, err := json.MarshalIndent(report, "", "    ") // pretty print

	if err != nil {
		return errors.Wrap(err, "Problem marshalling report into JSON")
	}
	// no directory, print to stdout
	if config.Opts.Directory == "" {
		fmt.Printf("%s\n", string(jsonReport))
		return nil
	}
	filepath = strings.Join([]string{config.Opts.Directory, filepath}, "/")
	grip.Infof("Printing the report to %s\n", filepath)
	file, err := os.Create(filepath)
	if err != nil {
		return errors.Wrap(err, "Problem creating file")
	}
	defer file.Close()
	_, err = file.Write(jsonReport)
	if err != nil {
		return err
	}
	return errors.Wrap(err, "Problem writing to file")
}
