package db

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/tychoish/sink"
	"gopkg.in/mgo.v2/bson"
)

func Insert(collection string, item interface{}) error {
	session, db, err := sink.GetMgoSession()
	if err != nil {
		return err
	}
	defer session.Close()

	return errors.Wrap(db.C(collection).Insert(item), "problem inserting result")
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

// ClearCollections clears all documents from all the specified collections, returning an error
// immediately if clearing any one of them fails.
func ClearCollections(collections ...string) error {
	session, db, err := sink.GetMgoSession()
	if err != nil {
		return err
	}
	defer session.Close()
	for _, collection := range collections {
		_, err = db.C(collection).RemoveAll(bson.M{})
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("couldn't clear collection: %v", collection))
		}
	}
	return nil
}

// Update updates one matching document in the collection.
func Update(collection string, query interface{}, update interface{}) error {
	session, db, err := sink.GetMgoSession()
	if err != nil {
		return err
	}
	defer session.Close()

	return db.C(collection).Update(query, update)
}

// FindAll finds the items from the specified collection and unmarshals them into the
// provided interface, which must be a slice.
func FindAll(collection string, query interface{},
	projection interface{}, sort []string, skip int, limit int,
	out interface{}) error {

	session, db, err := sink.GetMgoSession()
	if err != nil {
		return err
	}
	defer session.Close()

	q := db.C(collection).Find(query).Select(projection)
	if len(sort) != 0 {
		q = q.Sort(sort...)
	}
	return q.Skip(skip).Limit(limit).All(out)
}
