package log

import (
	"github.com/pkg/errors"
	"github.com/tychoish/sink/db"
	"github.com/tychoish/sink/db/bsonutil"
	"github.com/tychoish/sink/model"
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

	// parsed out information
	NumberLines int `bson:"lines"`

	model.Metadata `bson:"metadata"`
}

var (
	IDKey          = bsonutil.MustHaveTag(Log{}, "ID")
	LogIDKey       = bsonutil.MustHaveTag(Log{}, "LogID")
	URLKey         = bsonutil.MustHaveTag(Log{}, "URL")
	Segment        = bsonutil.MustHaveTag(Log{}, "Segment")
	NumberLinesKey = bsonutil.MustHaveTag(Log{}, "NumberLines")
	MetadataKey    = bsonutil.MustHaveTag(Log{}, "Metadata")
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
			MetadataKey + "." + model.ModKey: l.Metadata.Modifications,
		},
		bson.M{
			"$inc": bson.M{MetadataKey + "." + model.ModKey: 1},
			"$set": bson.M{NumberLinesKey: l.NumberLines},
		},
	))
}

func ByLogID(id string) db.Q {
	return db.Query(bson.M{LogIDKey: id})
}

func ByID(id string) db.Q {
	return db.Query(bson.M{IDKey: id})
}

func FindOne(query db.Q) (*Log, error) {
	l := &Log{}
	err := db.FindOneQ(LogsCollection, query, l)
	if err == mgo.ErrNotFound {
		return l, nil
	}
	return l, err
}
func FindAll(query db.Q) ([]Log, error) {
	logs := []Log{}
	err := db.FindAllQ(LogsCollection, query, &logs)
	if err == mgo.ErrNotFound {
		return logs, nil
	}
	return logs, err
}
