package cedar

import (
	"errors"
	"time"

	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/grip"
)

// Configuration defines
type Configuration struct {
	BucketName         string
	DatabaseName       string
	MongoDBURI         string
	MongoDBDialTimeout time.Duration
	UseLocalQueue      bool
	NumWorkers         int
	UserManager        gimlet.UserManager
	ExpireAfter        time.Duration
}

func (c *Configuration) Validate() error {
	catcher := grip.NewBasicCatcher()

	if c.MongoDBURI == "" {
		catcher.Add(errors.New("must specify a mongodb url"))
	}
	if c.NumWorkers < 1 {
		catcher.Add(errors.New("must specify a valid number of amboy workers"))
	}
	if c.MongoDBDialTimeout <= 0 {
		c.MongoDBDialTimeout = 2 * time.Second
	}
	if c.ExpireAfter <= 0 {
		c.ExpireAfter = time.Duration(365*24*60) * time.Minute
	}

	return catcher.Resolve()
}
