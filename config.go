package sink

import "github.com/evergreen-ci/sink/cost"

// Configuration defines
type Configuration struct {
	BucketName   string
	DatabaseName string

	Cost *cost.Config
}
