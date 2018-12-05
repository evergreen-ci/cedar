package model

import (
	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
	"github.com/pkg/errors"
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
	env       cedar.Environment
}

var (
	logRecordIDKey           = bsonutil.MustHaveTag(LogRecord{}, "LogID")
	logRecordURLKey          = bsonutil.MustHaveTag(LogRecord{}, "URL")
	logRecordKeyNameKey      = bsonutil.MustHaveTag(LogRecord{}, "KeyName")
	logRecordLastSegementKey = bsonutil.MustHaveTag(LogRecord{}, "LastSegment")
	logRecordMetadataKey     = bsonutil.MustHaveTag(LogRecord{}, "Metadata")
)

func (l *LogRecord) Setup(e cedar.Environment) { l.env = e }
func (l *LogRecord) IsNil() bool              { return !l.populated }
func (l *LogRecord) Save() error {
	if !l.populated {
		return errors.New("cannot insert a log segment that is not poulated")
	}

	conf, session, err := cedar.GetSessionWithConfig(l.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	return errors.WithStack(session.DB(conf.DatabaseName).C(logRecordCollection).Insert(l))
}

func (l *LogRecord) Find() error {
	conf, session, err := cedar.GetSessionWithConfig(l.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()
	l.populated = false

	err = session.DB(conf.DatabaseName).C(logRecordCollection).FindId(l.LogID).One(l)

	if db.ResultsNotFound(err) {
		return errors.Wrapf(err, "could not find document with id '%s'", l.LogID)
	} else if err != nil {
		return errors.Wrap(err, "problem running log query")
	}
	l.populated = true

	return nil
}
