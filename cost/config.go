package cost

import "github.com/evergreen-ci/sink/amazon"

// Config holds information from a user submitted cost config file.
type Config struct {
	Opts      Options           `yaml:"opts"`
	Pricing   *amazon.EBSPrices `yaml:"pricing"`
	Providers []*Provider       `yaml:"providers"`
	RootURL   string            `yaml:"root_url"`
	User      string            `yaml:"evergreen_user"`
	Key       string            `yaml:"evergreen_api_key"`
}

// Options holds user submitted default options for the cost tool.
type Options struct {
	Directory string `yaml:"directory"`
	Duration  string `yaml:"duration"`
}
