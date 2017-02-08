package db

import (
	mgo "gopkg.in/mgo.v2"
)

func Insert(collection string, item interface{}) error {
	session, db, err := sink.GetMgoSession()
	if err != nil {
		return err
	}
	defer session.Close()
	return db.C(collection).Insert(item)
}

// FindOne finds one item from the specified collection and unmarshals it into the
// provided interface, which must be a pointer.
func FindOne(collection string, query interface{},
	projection interface{}, sort []string, out interface{}) error {

	session, db, err := sink.GetMgoSession()
	if err != nil {
		return err
	}
	defer session.Close()

	q := db.C(collection).Find(query).Select(projection)
	if len(sort) != 0 {
		q = q.Sort(sort...)
	}
	return q.One(out)
}
