package cost

import (
	"github.com/evergreen-ci/sink/amazon"
	"github.com/evergreen-ci/sink/evergreen"
)

// Config holds information from a user submitted cost config file.
type Config struct {
	Opts          Options                  `yaml:"opts"`
	S3Info        *amazon.S3Info           `yaml:"s3_info"`
	EvergreenInfo *evergreen.EvergreenInfo `yaml:"evergreen_info"`
	Pricing       *amazon.EBSPrices        `yaml:"pricing"`
	Accounts      []string                 `yaml:"aws_accounts"`
	Providers     []*Provider              `yaml:"providers"`
}

// Options holds user submitted default options for the cost tool.
type Options struct {
	Directory string `yaml:"directory"`
	Duration  string `yaml:"duration"`
}
