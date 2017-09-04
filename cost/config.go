package cost

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/evergreen-ci/sink/amazon"
	"github.com/evergreen-ci/sink/evergreen"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

// Config holds information from a user submitted cost config file.
type Config struct {
	Opts      Options                 `bson:"options" json:"options" yaml:"options"`
	Evergreen evergreen.EvergreenInfo `bson:"evergreen" json:"evergreen" yaml:"evergreen"`
	Amazon    Amazon                  `bson:"aws" json:"aws" yaml:"aws"`
	Providers []Provider              `bson:"providers" json:"providers" yaml:"providers"`
}

type Amazon struct {
	Accounts  []string         `bson:"accounts" json:"accounts" yaml:"accounts"`
	S3Info    amazon.S3Info    `bson:"s3" json:"s3" yaml:"s3"`
	EBSPrices amazon.EBSPrices `bson:"ebs_pricing" json:"ebs_pricing" yaml:"ebs_pricing"`
}

// Options holds user submitted default options for the cost tool.
type Options struct {
	Directory string `bson:"directory" json:"directory" yaml:"directory"`
	Duration  string `bson:"duration" json:"duration" yaml:"duration"`
}

// LoadConfig takes a file path, reads it to YAML, and then converts
// it to a Config struct.
func LoadConfig(file string) (*Config, error) {
	yamlFile, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("invalid file: %s", file))
	}

	newConfig := &Config{}
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
