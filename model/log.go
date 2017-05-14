package model

import (
	"github.com/pkg/errors"
	"github.com/tychoish/sink/db"
	"github.com/tychoish/sink/db/bsonutil"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

const logRecordCollection = "simple.log.records"

// GOAL: the model package should export types with methods that wrap
//    up all required database interaction using functionality from the
//    database package. We should not export Key names or query builders

type LogRecord struct {
	LogID       string `bson:"_id"`
	URL         string `bson:"url"`
	LastSegment int    `bson:"seg"`
	Bucket      string `bson:"bucket"`
	KeyName     string `bson:"key"`
	Metadata    `bson:"metadata"`

	populated bool
}

var (
	logRecordIDKey           = bsonutil.MustHaveTag(LogRecord{}, "LogID")
	logRecordURLKey          = bsonutil.MustHaveTag(LogRecord{}, "URL")
	logRecordKeyNameKey      = bsonutil.MustHaveTag(LogRecord{}, "KeyName")
	logRecordLastSegementKey = bsonutil.MustHaveTag(LogRecord{}, "LastSegment")
	logRecordMetadataKey     = bsonutil.MustHaveTag(LogRecord{}, "Metadata")
)

func (l *LogRecord) IsNil() bool { return l.populated }

func (l *LogRecord) Insert() error {
	return errors.WithStack(db.Insert(logRecordCollection, l))
}

func (l *LogRecord) Find(id string) error {
	err := db.Query(bson.M{logRecordIDKey: id}).FindOne(logRecordCollection, l)

	l.populated = false
	if err == mgo.ErrNotFound {
		return nil
	}
	l.populated = true

	if err != nil {
		return errors.Wrap(err, "problem running log query")
	}

	return nil
}
