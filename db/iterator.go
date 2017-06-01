package db

import "github.com/tychoish/sink"

// ResultsIterator
type ResultsIterator interface {
	All(interface{}) error
	Next(interface{}) bool
	Close() error
	Err() error
}

func iter(collection string, query interface{}, project interface{}, sort []string, skip, limit int) ResultsIterator {
	session, db, err := sink.GetMgoSession()
	if err != nil {
		return nil
	}
	defer session.Close()

	q := db.C(collection).Find(query).Select(project)

	if len(sort) != 0 {
		q = q.Sort(sort...)
	}

	if skip != 0 {
		q = q.Skip(skip)
	}

	if limit != 0 {
		q = q.Limit(limit)
	}

	return q.Iter()
}
