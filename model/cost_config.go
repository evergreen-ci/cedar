package model

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/evergreen"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/anser/model"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

const (
	configurationCollection = "configuration"
	costReportingID         = "build-cost-reporting"
)

type CostConfig struct {
	ID        string                   `bson:"_id" json:"id" yaml:"id"`
	Opts      CostConfigOptions        `bson:"options" json:"options" yaml:"options"`
	Providers []CloudProvider          `bson:"providers" json:"providers" yaml:"providers"`
	Evergreen evergreen.ConnectionInfo `bson:"evergreen" json:"evergreen" yaml:"evergreen"`
	Amazon    CostConfigAmazon         `bson:"aws" json:"aws" yaml:"aws"`

	populated bool
	env       sink.Environment
}

var (
	costConfigOptsKey      = bsonutil.MustHaveTag(CostConfig{}, "Opts")
	costConfigProvidersKey = bsonutil.MustHaveTag(CostConfig{}, "Providers")
	costConfigEvergreenKey = bsonutil.MustHaveTag(CostConfig{}, "Evergreen")
	costConfigAmazonKey    = bsonutil.MustHaveTag(CostConfig{}, "Amazon")
)

type CostConfigOptions struct {
	Directory              string `bson:"directory,omitempty" json:"directory" yaml:"directory"`
	Duration               string `bson:"duration" json:"duration" yaml:"duration"`
	AllowIncompleteResults bool   `bson:"allow_incomplete" json:"allow_incomplete" yaml:"allow_incomplete"`
}

var (
	costConfigOptsDirectoryKey       = bsonutil.MustHaveTag(CostConfigOptions{}, "Directory")
	costConfigOptsDurationKey        = bsonutil.MustHaveTag(CostConfigOptions{}, "Duration")
	costConfigOptsAllowIncompleteKey = bsonutil.MustHaveTag(CostConfigOptions{}, "AllowIncompleteResults")
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
	} else if err != nil {
		return errors.Wrap(err, "problem finding cost config document")
	}
	c.populated = true

	return nil
}

func (c *CostConfig) Save() error {
	if err := c.Validate(); err != nil {
		return errors.WithStack(err)
	}

	if !c.populated {
		return errors.New("cannot save an non-populated cost configuration")
	}

	c.ID = costReportingID

	conf, session, err := sink.GetSessionWithConfig(c.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	changeInfo, err := session.DB(conf.DatabaseName).C(configurationCollection).UpsertId(costReportingID, c)
	grip.Debug(message.Fields{
		"ns":          model.Namespace{DB: conf.DatabaseName, Collection: configurationCollection},
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
// If the config file duration is empty, or less than one minute, we return the default.
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

func (c *CostConfig) Validate() error {
	var errs []string

	if !c.Amazon.EBSPrices.IsValid() {
		errs = append(errs, "Configuration file requires EBS pricing information")
	}
	if !c.Amazon.S3Info.IsValid() {
		errs = append(errs, "Configuration file requires S3 bucket information")
	}
	if !c.Evergreen.IsValid() {
		errs = append(errs, "Configuration file requires evergreen user, key, and rootURL")
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}

// LoadCostConfig takes a file path, reads it to YAML, and then converts
// it to a Config struct.
func LoadCostConfig(file string) (*CostConfig, error) {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return nil, errors.Errorf("file %s does not exist", file)
	}

	yamlFile, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("invalid file: %s", file))
	}

	newConfig := &CostConfig{}
	if err = yaml.Unmarshal(yamlFile, newConfig); err != nil {
		return nil, errors.Wrap(err, "invalid yaml format")
	}

	if err = newConfig.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid cost configuration")
	}

	newConfig.populated = true

	return newConfig, nil
}
