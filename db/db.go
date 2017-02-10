package db

import (
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

	return errors.WithStack(db.C(collection).Insert(item))
}

// FindOne finds one item from the specified collection and unmarshals it into the
// provided interface, which must be a pointer.
func FindOne(coll string, query, proj interface{}, sort []string, out interface{}) error {
	session, db, err := sink.GetMgoSession()
	if err != nil {
		return errors.Wrap(err, "problem getting session")
	}
	defer session.Close()

	q := db.C(coll).Find(query).Select(proj)
	if len(sort) != 0 {
		q = q.Sort(sort...)
	}

	return errors.WithStack(q.One(out))
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
			return errors.Wrapf(err, "couldn't clear collection: %s", collection)
		}
	}
	return nil
}

// Update updates one matching document in the collection.
func Update(collection string, query, update interface{}) error {
	session, db, err := sink.GetMgoSession()
	if err != nil {
		return err
	}
	defer session.Close()

	return errors.WithStack(db.C(collection).Update(query, update))
}

// FindAll finds the items from the specified collection and unmarshals them into the
// provided interface, which must be a slice.
func FindAll(coll string, query, proj interface{}, sort []string, skip, limit int, out interface{}) error {

	session, db, err := sink.GetMgoSession()
	if err != nil {
		return err
	}
	defer session.Close()

	q := db.C(coll).Find(query).Select(proj)
	if len(sort) != 0 {
		q = q.Sort(sort...)
	}
	return errors.WithStack(q.Skip(skip).Limit(limit).All(out))
}

func RemoveOne(coll string, id interface{}) error {
	session, db, err := sink.GetMgoSession()
	if err != nil {
		return err
	}
	defer session.Close()

	return errors.WithStack(db.C(coll).Remove(bson.M{"_id": id}))

}
