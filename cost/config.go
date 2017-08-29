package cost

import (
	"github.com/evergreen-ci/sink/amazon"
	"github.com/evergreen-ci/sink/evergreen"
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
