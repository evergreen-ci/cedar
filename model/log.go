package model

import "github.com/tychoish/sink/db"

const (
	LogsCollection = "logs"
)

type Log struct {
	// common log information
	LogID   string `bson:"log_id"`
	URL     string `bson:"url"`
	Segment int    `bson:"seg"`

	// parsed out information
	NumberLines int `bson:"lines"`
}

func (l *Log) Insert() error {
	return errors.WithStack(db.Insert(LogsCollection, l))
}
