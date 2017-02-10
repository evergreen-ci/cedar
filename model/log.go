package model

import (
	"github.com/pkg/errors"
	"github.com/tychoish/sink/db"
	"github.com/tychoish/sink/db/bsonutil"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	LogsCollection = "logs"
)

type Log struct {
	// common log information
	ID      bson.ObjectId `bson:"_id"`
	LogID   string        `bson:"log_id"`
	URL     string        `bson:"url"`
	Segment int           `bson:"seg"`
	Bucket  string        `bson:"bucket"`
	KeyName string        `bson:"key"`

	// parsed out information
	Metrics LogMetrics `bson:"metrics"`

	Metadata `bson:"metadata"`
}

type LogMetrics struct {
	NumberLines       int            `bson:"lines"`
	LetterFrequencies map[string]int `bson:"frequencies"`
	UniqueLetters     int            `bson:"letters"`
}

var (
	IDKey                = bsonutil.MustHaveTag(Log{}, "ID")
	LogIDKey             = bsonutil.MustHaveTag(Log{}, "LogID")
	URLKey               = bsonutil.MustHaveTag(Log{}, "URL")
	SegmentKey           = bsonutil.MustHaveTag(Log{}, "Segment")
	MetricsKey           = bsonutil.MustHaveTag(Log{}, "Metrics")
	MetadataKey          = bsonutil.MustHaveTag(Log{}, "Metadata")
	NumberLinesKey       = bsonutil.MustHaveTag(LogMetrics{}, "NumberLines")
	LetterFrequenciesKey = bsonutil.MustHaveTag(LogMetrics{}, "LetterFrequencies")
)

func (l *Log) Insert() error {
	if l.ID == "" {
		l.ID = bson.NewObjectId()
	}

	return errors.WithStack(db.Insert(LogsCollection, l))
}

func (l *Log) SetNumberLines(n int) error {
	// find the log, check the version
	// modify the log, save it
	return errors.WithStack(db.Update(LogsCollection,
		bson.M{
			IDKey: l.ID,
			MetadataKey + "." + ModKey: l.Metadata.Modifications,
		},
		bson.M{
			"$inc": bson.M{MetadataKey + "." + ModKey: 1},
			"$set": bson.M{MetricsKey + "." + NumberLinesKey: l.Metrics.NumberLines},
		},
	))
}

func ByLogID(id string) db.Q {
	return db.Query(bson.M{LogIDKey: id})
}

func ByID(id string) db.Q {
	return db.Query(bson.M{IDKey: id})
}

func FindOneLog(query db.Q) (*Log, error) {
	l := &Log{}
	err := db.FindOneQ(LogsCollection, query, l)
	if err == mgo.ErrNotFound {
		return l, nil
	}
	return l, err
}
func FindAllLogs(query db.Q) ([]Log, error) {
	logs := []Log{}
	err := db.FindAllQ(LogsCollection, query, &logs)
	if err == mgo.ErrNotFound {
		return logs, nil
	}
	return logs, err
}
