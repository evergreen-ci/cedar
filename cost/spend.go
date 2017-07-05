package cost

import (
	"fmt"
	"io/ioutil"
	"math"
	"time"

	"github.com/evergreen-ci/sink/amazon"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type timeRange struct {
	start time.Time
	end   time.Time
}

// GetGranularity returns the granularity in the config file as type time.Duration.
// If the config file duration is empty, we return the default.
func (c *Config) GetGranularity() (time.Duration, error) {
	configDur := c.Opts.Duration
	var err error
	granularity := 4 * time.Hour //default value
	if configDur != "0s" {
		granularity, err = time.ParseDuration(configDur)
		if err != nil {
			return 0, errors.Wrap(err, fmt.Sprintf("Could not parse duration %s", configDur))
		}
	}
	return granularity, nil
}

// UpdateSpendProviders updates the given config file's providers to include
// the providers in the given Provider array. If the provider already exists,
// we update the cost.
func (c *Config) UpdateSpendProviders(newProv []*Provider) {
	// Iterate through new providers, and if equal to old provider, update cost.
	// Else, add to list of providers.
	for _, new := range newProv {
		added := false
		for _, old := range c.Providers {
			if new.Name == old.Name {
				old.Cost = new.Cost
				added = true
				break
			}
		}
		if !added {
			c.Providers = append(c.Providers, new)
		}
	}
}

// GetStartTime takes in a string of the form "YYYY-MM-DDTHH:MM" as the start
// time for the report, and converts this to time.Time type. If the given string
// is empty, we instead default to using the current time minus the granularity.
func GetStartTime(s string, granularity time.Duration) (time.Time, error) {
	const layout = "2006-01-02T15:04" //Using reference Mon Jan 2 15:04:05 -0700 MST 2006
	var startTime time.Time
	var err error
	if s != "" {
		startTime, err = time.Parse(layout, s)
		if err != nil {
			return startTime, errors.Wrap(err, "incorrect start format: "+
				"should be YYYY-MM-DDTHH:MM")
		}
	} else {
		startTime = time.Now().Add(-granularity)
	}
	return startTime, nil
}

// YAMLToConfig takes a file path, reads it to YAML, and then converts it to a Config struct.
func YAMLToConfig(file string) (*Config, error) {
	newConfig := &Config{}
	yamlFile, err := ioutil.ReadFile(file)
	if err != nil {
		return newConfig, errors.Wrap(err, fmt.Sprintf("invalid file: %s", file))
	}

	err = yaml.Unmarshal(yamlFile, newConfig)
	if err != nil {
		return newConfig, errors.Wrap(err, "invalid yaml format")
	}
	return newConfig, nil
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
// The sums are calculated from the information in the ec2Item array.
func (res *Item) setSums(items []*amazon.EC2Item) {
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
		res.TotalHours += int(item.Uptime)
	}
}

// avgItems sets the average price, fixed price, and uptime of the given cost item.
// The averages are calculated from the information in the ec2Item array.
func (res *Item) setAverages(items []*amazon.EC2Item) {
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
	res.AvgPrice = float32(avg(prices))
	res.FixedPrice = float32(avg(fixedPrices))
	res.AvgUptime = float32(avg(uptimes))
}

// createItemFromEC2Instance creates a new cost.Item using a key/item array pair.
func createItemFromEC2Instance(key *amazon.ItemKey, items []*amazon.EC2Item) *Item {
	item := &Item{
		Name:     key.Name,
		ItemType: string(key.ItemType),
	}
	item.setSums(items)
	item.setAverages(items)

	return item
}

// getAccounts takes in a range for the report, and returns an array of accounts
// containing EC2 instances.
func getAccounts(reportRange timeRange) ([]*Account, error) {
	awsReportRange := amazon.TimeRange{
		Start: reportRange.start,
		End:   reportRange.end,
	}
	client := amazon.NewClient()
	accounts, err := client.GetEC2Instances(awsReportRange)
	if err != nil {
		return nil, errors.Wrap(err, "Problem getting EC2 instances")
	}
	var accountReport []*Account
	for owner, instances := range accounts {
		service := &Service{
			Name: "ec2",
		}
		for key, items := range instances {
			item := createItemFromEC2Instance(key, items)
			service.Items = append(service.Items, item)
		}
		account := &Account{
			Name:     owner,
			Services: []*Service{service},
		}
		accountReport = append(accountReport, account)
	}
	return accountReport, nil
}
