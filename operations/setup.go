package operations

import (
	"time"

	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/model"
	"github.com/evergreen-ci/sink/units"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/queue"
	"github.com/mongodb/amboy/queue/driver"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	mgo "gopkg.in/mgo.v2"
)

func configure(numWorkers int, localQueue bool, mongodbURI, bucket, dbName string) error {
	sink.SetConf(&sink.Configuration{
		BucketName:   bucket,
		DatabaseName: dbName,
	})

	if localQueue {
		q := queue.NewLocalLimitedSize(numWorkers, 1024)
		grip.Infof("configured local queue with %d workers", numWorkers)
		if err := sink.SetQueue(q); err != nil {
			return errors.Wrap(err, "problem configuring queue")
		}
	} else {
		q := queue.NewRemoteUnordered(numWorkers)
		opts := driver.MongoDBOptions{
			URI:      mongodbURI,
			DB:       dbName,
			Priority: true,
		}

		mongoDriver := driver.NewMongoDB(sink.QueueName, opts)
		if err := q.SetDriver(mongoDriver); err != nil {
			return errors.Wrap(err, "problem configuring driver")
		}

		if err := sink.SetQueue(q); err != nil {
			return errors.Wrap(err, "problem caching queue")
		}

		grip.Info(message.MakeFieldsMessage("configured a remote mongodb-backed queue",
			message.Fields{"db": dbName, "prefix": sink.QueueName, "priority": true}))
	}

	// create and cache a db session for use in tasks
	session, err := mgo.Dial(mongodbURI)
	if err != nil {
		return errors.Wrapf(err, "could not connect to db %s", mongodbURI)
	}
	if err = sink.SetMgoSession(session); err != nil {
		return errors.Wrap(err, "problem caching DB session")
	}

	sender, err := model.NewDBSender("sink")
	if err != nil {
		return errors.Wrapf(err, "problem setting system sender")
	}

	if err = sink.SetSystemSender(sender); err != nil {
		return errors.Wrap(err, "problem setting cached log sender")
	}

	return nil
}

func backgroundJobs(ctx context.Context) error {
	// TODO: develop a specification format, either here or in
	// amboy so that you can specify a list of amboy.QueueOperation
	// functions + specific intervals
	//
	// In the mean time, we'll just register intervals here, and
	// hard code the configuration

	q, err := sink.GetQueue()
	if err != nil {
		return errors.Wrap(err, "problem fetching queue")
	}

	// This isn't how we'd do this in the long term, but I want to
	// have one job running on an interval
	var count int
	amboy.PeriodicQueueOperation(ctx, q, func(cue amboy.Queue) error {
		name := "periodic-poc"
		count++
		j := units.NewHelloWorldJob(name)
		err = cue.Put(j)
		grip.Error(message.NewErrorWrap(err,
			"problem scheduling job %s (count: %d)", name, count))

		return err
	}, time.Minute, true)

	return nil
}

// configureSpend calls the sink functions to configure spend
func configureSpend(path string) error {
	return sink.SetSpendConfig(path)
}
