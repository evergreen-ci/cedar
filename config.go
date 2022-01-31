package cedar

import (
	"time"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/queue"
	"github.com/mongodb/grip"
)

// Configuration defines configuration settings to initialize the global
// environment without the presence of a database to store the settings.
type Configuration struct {
	BucketName              string
	DatabaseName            string
	QueueName               string
	MongoDBURI              string
	MongoDBDialTimeout      time.Duration
	SocketTimeout           time.Duration
	DisableLocalQueue       bool
	DisableRemoteQueue      bool
	DisableRemoteQueueGroup bool
	DisableCache            bool
	NumWorkers              int
	DBUser                  string
	DBPwd                   string
}

// Validate checks that all the required fields are set and sets defaults for
// unset optional fields.
func (c *Configuration) Validate() error {
	catcher := grip.NewBasicCatcher()

	if c.MongoDBURI == "" {
		catcher.New("must specify a mongodb url")
	}
	if c.NumWorkers < 1 {
		catcher.New("must specify a valid number of amboy workers")
	}
	if c.MongoDBDialTimeout <= 0 {
		c.MongoDBDialTimeout = 2 * time.Second
	}
	if c.SocketTimeout <= 0 {
		c.SocketTimeout = time.Minute
	}
	if c.QueueName == "" {
		c.QueueName = "cedar.service"
	}

	return catcher.Resolve()
}

// GetQueueOptions returns the options to initialize a MongoDB-backed Amboy
// queue.
func (c *Configuration) GetQueueOptions() queue.MongoDBOptions {
	return queue.MongoDBOptions{
		URI:                      c.MongoDBURI,
		DB:                       c.DatabaseName,
		Collection:               c.QueueName,
		CheckWaitUntil:           true,
		Format:                   amboy.BSON2,
		WaitInterval:             time.Second,
		SkipQueueIndexBuilds:     true,
		SkipReportingIndexBuilds: true,
		SampleSize:               300,
	}
}

// GetQueueGroupOptions returns the options to initialize a MongoDB-backed Amboy
// queue group.
func (c *Configuration) GetQueueGroupOptions() queue.MongoDBOptions {
	opts := c.GetQueueOptions()
	opts.UseGroups = true
	opts.GroupName = c.QueueName
	return opts
}

// HasAuth returns whether or not the database uses authentication.
func (c *Configuration) HasAuth() bool {
	return c.DBUser != ""
}
