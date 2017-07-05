package cost

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

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
