package model

import (
	"github.com/pkg/errors"
	"github.com/tychoish/grip"
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
	SgmentURLKey         = bsonutil.MustHaveTag(LogSegment{}, "URL")
	SegmentKeyNameKey    = bsonutil.MustHaveTag(LogSegment{}, "KeyName")
	SegmentIDKey         = bsonutil.MustHaveTag(LogSegment{}, "Segment")
	SegmentMetricsKey    = bsonutil.MustHaveTag(LogSegment{}, "Metrics")
	SegmentMetadataKey   = bsonutil.MustHaveTag(LogSegment{}, "Metadata")
	NumberLinesKey       = bsonutil.MustHaveTag(LogMetrics{}, "NumberLines")
	UniqueLetters        = bsonutil.MustHaveTag(LogMetrics{}, "UniqueLetters")
	LetterFrequenciesKey = bsonutil.MustHaveTag(LogMetrics{}, "LetterFrequencies")
)

func (l *LogSegment) Insert() error {
	if l.ID == "" {
		l.ID = bson.NewObjectId()
	}

	return errors.WithStack(db.Insert(LogSegmentsCollection, l))
}

func (l *LogSegment) Find(query *db.Q) error {
	err := query.FindOne(LogSegmentsCollection, l)
	if err == mgo.ErrNotFound {
		return nil
	}

	if err != nil {
		return errors.Wrapf(err, "problem running log query %+v", query)
	}

	return nil
}

func (l *LogSegment) Remove() error {
	return db.RemoveOne(LogSegmentsCollection, l.ID)
}

func (l *LogSegments) Find(query *db.Q) error {
	err := query.FindAll(LogSegmentsCollection, l)
	// fmt.Println("--->", err, len(l))
	if err != nil && err != mgo.ErrNotFound {
		return errors.Wrapf(err, "problem running log query %+v", query)
	}
	return nil
}

func (l *LogSegments) LogSegments() []LogSegment {
	ret := []LogSegment(*l)
	grip.Alertln(len(ret), "::", l)

	return ret
}

func (l *LogSegment) SetNumberLines(n int) error {
	// find the log, check the version
	// modify the log, save it
	return errors.WithStack(db.Update(LogSegmentsCollection,
		bson.M{
			IDKey: l.ID,
			SegmentMetadataKey + "." + ModKey: l.Metadata.Modifications,
		},
		bson.M{
			"$set": bson.M{
				SegmentMetadataKey + "." + ModKey:        l.Metrics.NumberLines + 1,
				SegmentMetricsKey + "." + NumberLinesKey: l.Metrics.NumberLines,
			},
		},
	))
}

func ByLogID(id string) *db.Q {
	return db.StringKeyQuery(LogIDKey, id)
}
