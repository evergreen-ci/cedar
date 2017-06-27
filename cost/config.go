package cost

// Config holds information from a user submitted cost config file.
type Config struct {
	Opts      Options     `yaml:"opts"`
	Providers []*Provider `yaml:"providers"`
}

// Options holds user submitted default options for the cost tool.
type Options struct {
	Directory string `yaml:"directory"`
	Duration  string `yaml:"duration"`
}
