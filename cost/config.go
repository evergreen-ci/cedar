package cost

// Config holds information from a user submitted cost config file.
type Config struct {
	Opts      Options    `yaml:"opts,omitempty"`
	Providers []Provider `yaml:"providers,omitempty"`
}

// Options holds user submitted default options for the cost tool.
type Options struct {
	Directory string `yaml:"directory, omitempty"`
	Duration  string `yaml:"duration, omitempty"`
}
