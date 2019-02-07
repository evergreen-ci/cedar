package cedar

import (
	"errors"
	"time"

	"github.com/mongodb/amboy/queue"
	"github.com/mongodb/grip"
)

// Configuration defines
type Configuration struct {
	BucketName         string
	DatabaseName       string
	QueueDatabaseName  string
	MongoDBURI         string
	MongoDBDialTimeout time.Duration
	SocketTimeout      time.Duration
	DisableLocalQueue  bool
	DisableRemoteQueue bool
	NumWorkers         int
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
	if c.SocketTimeout <= 0 {
		c.SocketTimeout = time.Minute
	}
	if c.QueueDatabaseName == "" {
		c.QueueDatabaseName = "amboy"
	}

	return catcher.Resolve()
}

func (c *Configuration) GetQueueOptions() queue.MongoDBOptions {
	return queue.MongoDBOptions{
		URI:            c.MongoDBURI,
		DB:             c.QueueDatabaseName,
		Priority:       true,
		CheckWaitUntil: true,
	}

}
