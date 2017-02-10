package model

import (
	"github.com/pkg/errors"
	"github.com/tychoish/sink/db"
	"github.com/tychoish/sink/db/bsonutil"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	LogSegmentsCollection = "simple.log.segments"
)

type LogSegments []LogSegment

type LogSegment struct {
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
	UniqueLetters     int            `bson:"letters"`
	LetterFrequencies map[string]int `bson:"frequencies"`
}

var (
	IDKey                = bsonutil.MustHaveTag(LogSegment{}, "ID")
	LogIDKey             = bsonutil.MustHaveTag(LogSegment{}, "LogID")
	URLKey               = bsonutil.MustHaveTag(LogSegment{}, "URL")
	KeyNameKey           = bsonutil.MustHaveTag(LogSegment{}, "KeyName")
	SegmentKey           = bsonutil.MustHaveTag(LogSegment{}, "Segment")
	MetricsKey           = bsonutil.MustHaveTag(LogSegment{}, "Metrics")
	MetadataKey          = bsonutil.MustHaveTag(LogSegment{}, "Metadata")
	NumberLinesKey       = bsonutil.MustHaveTag(LogMetrics{}, "NumberLines")
	UniqueLetters        = bsonutil.MustHaveTag(LogMetrics{}, "UniqueLetters")
	LetterFrequenciesKey = bsonutil.MustHaveTag(LogMetrics{}, "LetterFrequencies")
)

func (l *LogSegment) Insert() error {
	if l.ID == "" {
		l.ID = bson.NewObjectId()
	}

	return errors.WithStack(db.Insert(LogSegementsCollection, l))
}

func (l *LogSegment) Find(query *db.Q) error {
	err := query.FindOne(LogSegementsCollection, l)
	if err == mgo.ErrNotFound {
		return nil
	}

	if err != nil {
		return errors.Wrapf(err, "problem running log query %+v", query)
	}

	return nil
}

func (l *LogSegment) Remove() error {
	db.RemoveOne(LogSegmentsCollection, l.ID)
}

func (l LogSegments) Find(query *db.Q) error {
	if len(l) > 0 {
		l = LogSegments{}
	}

	err := query.FindAll(LogSegementsCollection, &l)
	if err != nil && err != mgo.ErrNotFound {
		return errors.Wrapf(err, "problem running log query %+v", query)
	}

	return nil

}

func (l LogSegments) LogSegments() []LogSegment {
	return []LogSegment(l)
}

func (l *LogSegment) SetNumberLines(n int) error {
	// find the log, check the version
	// modify the log, save it
	return errors.WithStack(db.Update(LogSegementsCollection,
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

func ByLogID(id string) *db.Q {
	return db.StringKeyQuery(LogIDKey, id)
}
