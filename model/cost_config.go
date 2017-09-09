package model

import (
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/amazon"
	"github.com/evergreen-ci/sink/bsonutil"
	"github.com/evergreen-ci/sink/evergreen"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"github.com/tychoish/anser/db"
	yaml "gopkg.in/yaml.v2"
)

const (
	configurationCollection = "configuration"
	costReportingID         = "build-cost-reporting"
)

type CostConfig struct {
	ID   string `bson:"_id" json:"id" yaml:"id"`
	Opts struct {
		// Options holds user submitted default options for the cost tool.
		Directory string `bson:"directory,omitempty" json:"directory" yaml:"directory"`
		Duration  string `bson:"duration" json:"duration" yaml:"duration"`
	} `bson:"options" json:"options" yaml:"options"`
	Providers []struct {
		Name string  `json:"name,omitempty" yaml:"name,omitempty"`
		Cost float32 `json:"cost,omitempty" yaml:"cost,omitempty"`
	} `bson:"providers" json:"providers" yaml:"providers"`
	Evergreen evergreen.ConnectionInfo `bson:"evergreen" json:"evergreen" yaml:"evergreen"`
	Amazon    struct {
		Accounts  []string         `bson:"accounts" json:"accounts" yaml:"accounts"`
		S3Info    amazon.S3Info    `bson:"s3" json:"s3" yaml:"s3"`
		EBSPrices amazon.EBSPrices `bson:"ebs_pricing" json:"ebs_pricing" yaml:"ebs_pricing"`
	} `bson:"aws" json:"aws" yaml:"aws"`

	populated bool
	env       sink.Environment
}

var (
	costConfigOptsKey      = bsonutil.MustHaveTag(CostConfig{}, "Opts")
	costConfigProvidersKey = bsonutil.MustHaveTag(CostConfig{}, "Providers")
	costConfigEvergreenKey = bsonutil.MustHaveTag(CostConfig{}, "Evergreen")
	costConfigAmazonKey    = bsonutil.MustHaveTag(CostConfig{}, "Amazon")

	// costConfigOptsDirectoryKey   = bsonutil.MustHaveTag(CostConfig{}.Opts, "Directory")
	// costConfigOptsDurationKey    = bsonutil.MustHaveTag(CostConfig{}.Opts, "Duration")
	// costConfigProvidersNameKey   = bsonutil.MustHaveTag(CostConfig{}.Providers, "Name")
	// costConfigProvidersCostKey   = bsonutil.MustHaveTag(CostConfig{}.Providers, "Cost")
	// costConfigAmazonAccountsKey  = bsonutil.MustHaveTag(CostConfig{}.Amazon, "Accounts")
	// costConfigAmazonS3InfoKey    = bsonutil.MustHaveTag(CostConfig{}.Amazon, "S3Info")
	// costConfigAmazonEBSPricesKey = bsonutil.MustHaveTag(CostConfig{}.Amazon, "EBSPrices")
)

func (c *CostConfig) Setup(e sink.Environment) { c.env = e }
func (c *CostConfig) IsNil() bool              { return c.populated }
func (c *CostConfig) Find() error {
	conf, session, err := sink.GetSessionWithConfig(c.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	c.populated = false
	err = session.DB(conf.DatabaseName).C(configurationCollection).FindId(costReportingID).One(c)
	if db.ResultsNotFound(err) {
		return errors.New("could not find cost reporting document in the database")
	}
	c.populated = true

	return errors.Wrap(err, "problem finding cost config document")
}

func (c *CostConfig) Save() error {
	// TODO call validate here so we don't save junk data accidentally.

	c.ID = costReportingID

	conf, session, err := sink.GetSessionWithConfig(c.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	changeInfo, err := session.DB(conf.DatabaseName).C(configurationCollection).UpsertId(costReportingID, c)
	grip.Debug(message.Fields{
		"ns":          fmt.Sprintf("%s.%s", conf.DatabaseName, configurationCollection),
		"id":          costReportingID,
		"operation":   "save build cost reporting configuration",
		"change-info": changeInfo,
	})
	if db.ResultsNotFound(err) {
		return errors.New("could not find cost reporting document in the database")
	}

	return errors.Wrap(err, "problem saving cost reporting configuration")
}

// GetDuration returns the duration in the config file as type time.Duration.
// If the config file duration is empty, we return the default.
func (c *CostConfig) GetDuration(duration time.Duration) (time.Duration, error) {
	if duration < time.Minute {
		grip.Warningf("input time is %s, falling back to the config file or default", duration)
		duration = time.Hour
	}

	configDur := c.Opts.Duration
	var err error
	if configDur != "" {
		duration, err = time.ParseDuration(configDur)
		if err != nil {
			return 0, errors.Wrapf(err, "Could not parse duration %s", configDur)
		}
	}

	return duration, nil
}

// LoadCostConfig takes a file path, reads it to YAML, and then converts
// it to a Config struct.
func LoadCostConfig(file string) (*CostConfig, error) {
	yamlFile, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("invalid file: %s", file))
	}

	newConfig := &CostConfig{}
	if err = yaml.Unmarshal(yamlFile, newConfig); err != nil {
		return nil, errors.Wrap(err, "invalid yaml format")
	}

	var errs []string
	fmt.Println(newConfig.Amazon.EBSPrices)
	if !newConfig.Amazon.EBSPrices.IsValid() {
		errs = append(errs, "Configuration file requires EBS pricing information")
	}
	if !newConfig.Amazon.S3Info.IsValid() {
		errs = append(errs, "Configuration file requires S3 bucket information")
	}
	if !newConfig.Evergreen.IsValid() {
		errs = append(errs, "Configuration file requires evergreen user, key, and rootURL")
	}

	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "; "))
	}

	return newConfig, nil
}
