package model

import (
	"github.com/tychoish/sink/db"
	"github.com/tychoish/sink/db/bsonutil"
)

var LogsCollection = "simple.log.records"

type LogRecord struct {
	LogID       string `bson:"_id"`
	URL         string `bson:"url"`
	LastSegment int    `bson:"seg"`
	Bucket      string `bson:"bucket"`
	KeyName     string `bson:"key"`
	Metadata    `bson:"metadata"`
}

var (
	LogRecordIDKey  = bsonutil.MustHaveTag(LogRecord{}, "LogID")
	URLKey          = bsonutil.MustHaveTag(LogRecord{}, "URL")
	KeyNameKey      = bsonutil.MustHaveTag(LogRecord{}, "KeyName")
	LastSegementKey = bsonutil.MustHaveTag(LogRecord{}, "LastSegment")
	MetadataKey     = bsonutil.MustHaveTag(LogRecord{}, "Metadata")
)

func (l *LogRecord) Insert() error {
	return errors.WithStack(db.Insert(LogsCollection, l))
}

func (l *LogRecord) Find(query *db.Q) error {
	err := query.FindOne(LogRecord, l)
	// if err == mgo.ErrNotFound {
	// 	return nil
	// }

	if err != nil {
		return errors.Wrapf(err, "problem running log query %+v", query)
	}

	return nil
}
