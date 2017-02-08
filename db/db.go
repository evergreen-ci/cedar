package db

import (
	"github.com/pkg/errors"
	"github.com/tychoish/sink"
)

func Insert(collection string, item interface{}) error {
	session, db, err := sink.GetMgoSession()
	if err != nil {
		return err
	}
	defer session.Close()

	return errors.Wrap(db.C(collection).Insert(item), "problem inserting restul")
}

// FindOne finds one item from the specified collection and unmarshals it into the
// provided interface, which must be a pointer.
func FindOne(coll string, query interface{}, proj interface{}, sort []string, out interface{}) error {
	session, db, err := sink.GetMgoSession()
	if err != nil {
		return errors.Wrap(err, "problem getting session")
	}
	defer session.Close()

	q := db.C(coll).Find(query).Select(proj)
	if len(sort) != 0 {
		q = q.Sort(sort...)
	}

	return errors.Wrap(q.One(out), "problem resolving results")
}
