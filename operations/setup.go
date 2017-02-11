package operations

import (
	"github.com/mongodb/amboy/queue"
	"github.com/mongodb/amboy/queue/driver"
	"github.com/pkg/errors"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/sink"
	"github.com/tychoish/sink/model"
	mgo "gopkg.in/mgo.v2"
)

func configure(numWorkers int, localQueue bool, mongodbURI, bucket, dbName string) error {
	sink.SetConf(&sink.SinkConfiguration{
		BucketName:   bucket,
		DatabaseName: dbName,
	})

	if localQueue {
		q := queue.NewLocalUnordered(numWorkers)
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

		if err := sink.SetDriverOpts(sink.QueueName, opts); err != nil {
			return errors.Wrap(err, "problem caching queue driver options")
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
	if err := sink.SetMgoSession(session); err != nil {
		return errors.Wrap(err, "problem caching DB session")
	}

	sender, err := model.NewDBSender("sink")
	if err != nil {
		return errors.Wrapf(err, "problem setting system sender")
	}
	sink.SetSystemSender(sender)

	return nil
}
