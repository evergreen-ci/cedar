package model

import (
	"strings"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/anser/model"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

const (
	configurationCollection = "configuration"
	costReportingID         = "build-cost-reporting"
)

type CostConfig struct {
	ID        string                  `bson:"_id" json:"id" yaml:"id"`
	Opts      CostConfigOptions       `bson:"options" json:"options" yaml:"options"`
	Providers []CloudProvider         `bson:"providers" json:"providers" yaml:"providers"`
	Evergreen EvergreenConnectionInfo `bson:"evergreen" json:"evergreen" yaml:"evergreen"`
	Amazon    CostConfigAmazon        `bson:"aws" json:"aws" yaml:"aws"`

	populated bool
	env       cedar.Environment
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

func (c *CostConfig) Setup(e cedar.Environment) { c.env = e }
func (c *CostConfig) IsNil() bool               { return !c.populated }
func (c *CostConfig) Find() error {
	conf, session, err := cedar.GetSessionWithConfig(c.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	c.populated = false
	err = session.DB(conf.DatabaseName).C(configurationCollection).FindId(costReportingID).One(c)
	if db.ResultsNotFound(err) {
		return errors.Errorf("could not find cost reporting document [%s] in the database [%s.%s]",
			costReportingID, conf.DatabaseName, configurationCollection)
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
		return errors.New("cannot save non-populated cost configuration")
	}

	c.ID = costReportingID

	conf, session, err := cedar.GetSessionWithConfig(c.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	changeInfo, err := session.DB(conf.DatabaseName).C(configurationCollection).UpsertId(costReportingID, c)
	grip.Debug(message.Fields{
		"ns":          model.Namespace{DB: conf.DatabaseName, Collection: configurationCollection},
		"id":          costReportingID,
		"operation":   "save build cost reporting configuration",
		"change_info": changeInfo,
	})

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

// EvergreenConnectionInfo stores the root URL, username, and API key for the user
type EvergreenConnectionInfo struct {
	RootURL string `bson:"url" json:"url" yaml:"url"`
	User    string `bson:"user" json:"user" yaml:"user"`
	Key     string `bson:"key" json:"key" yaml:"key"`
}

var (
	costEvergreenConnInfoRootURLKey = bsonutil.MustHaveTag(EvergreenConnectionInfo{}, "RootURL")
	costEvergreenConnInfoUserKey    = bsonutil.MustHaveTag(EvergreenConnectionInfo{}, "User")
	costEvergreenConnInfoKeyKey     = bsonutil.MustHaveTag(EvergreenConnectionInfo{}, "Key")
)

// IsValid checks that a user, API key, and root URL are given in the
// ConnectionInfo structure
func (e *EvergreenConnectionInfo) IsValid() bool {
	if e == nil {
		return false
	}
	if e.RootURL == "" || e.User == "" || e.Key == "" {
		return false
	}
	return true
}

// LoadCostConfig takes a file path, reads it to YAML, and then converts
// it to a Config struct.
func LoadCostConfig(file string) (*CostConfig, error) {
	newConfig := &CostConfig{}
	if err := utility.ReadYAMLFile(file, newConfig); err != nil {
		return nil, errors.WithStack(err)
	}

	if err := newConfig.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid cost configuration")
	}

	newConfig.populated = true

	return newConfig, nil
}
