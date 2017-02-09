package model

import (
	"github.com/pkg/errors"
	"github.com/tychoish/sink/db"
	"github.com/tychoish/sink/db/bsonutil"
	"gopkg.in/mgo.v2/bson"
)

var (
	LogsCollection = "logs"
)

type Log struct {
	// common log information
	Id      string `bson:"_id"`
	LogID   string `bson:"log_id"`
	URL     string `bson:"url"`
	Segment int    `bson:"seg"`

	// parsed out information
	NumberLines int `bson:"lines"`

	Metadata `bson:"metadata"`
}

var (
	IdKey          = bsonutil.MustHaveTag(Log{}, "Id")
	LogIDKey       = bsonutil.MustHaveTag(Log{}, "LogID")
	URLKey         = bsonutil.MustHaveTag(Log{}, "URL")
	Segment        = bsonutil.MustHaveTag(Log{}, "Segment")
	NumberLinesKey = bsonutil.MustHaveTag(Log{}, "NumberLines")
	MetadataKey    = bsonutil.MustHaveTag(Log{}, "Metadata")
)

func (l *Log) Insert() error {
	return errors.WithStack(db.Insert(LogsCollection, l))
}

func (l *Log) SetNumberLines(n int) error {
	// find the log, check the version
	// modify the log, save it
	return errors.WithStack(db.Update(LogsCollection,
		bson.M{
			IdKey: l.Id,
			MetadataKey + "." + ModKey: l.Metadata.Modifications,
		},
		bson.M{
			"$inc": bson.M{MetadataKey + "." + ModKey: 1},
			"$set": bson.M{NumberLinesKey: l.NumberLines},
		},
	))
}
