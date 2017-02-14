package db

import (
	"github.com/pkg/errors"
	"github.com/tychoish/sink"
	"gopkg.in/mgo.v2/bson"
)

// PROPOSAL: these functions should be private with exposed
//    functionality via methods on the db.Q type.

// Insert inserts a document into a collection.
func Insert(collection string, item interface{}) error {
	session, db, err := sink.GetMgoSession()
	if err != nil {
		return errors.Wrap(err, "problem getting session")
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
		return errors.Wrap(err, "problem getting session")
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
		return errors.Wrap(err, "problem getting session")
	}
	defer session.Close()

	return errors.WithStack(db.C(collection).Update(query, update))
}

// UpdateID updates one _id-matching document in the collection.
func UpdateID(collection string, id, update interface{}) error {
	session, db, err := sink.GetMgoSession()
	if err != nil {
		return errors.Wrap(err, "problem getting session")
	}
	defer session.Close()

	return errors.WithStack(db.C(collection).UpdateId(id, update))
}

// FindAll finds the items from the specified collection and unmarshals them into the
// provided interface, which must be a slice.
func FindAll(coll string, query, proj interface{}, sort []string, skip, limit int, out interface{}) error {

	session, db, err := sink.GetMgoSession()
	if err != nil {
		return errors.Wrap(err, "problem getting session")
	}
	defer session.Close()

	q := db.C(coll).Find(query).Select(proj)
	if len(sort) != 0 {
		q = q.Sort(sort...)
	}
	return errors.WithStack(q.Skip(skip).Limit(limit).All(out))
}

// RemoveOne removes a single document from a collection that has the
// specified _id field.
func RemoveOne(coll string, id interface{}) error {
	session, db, err := sink.GetMgoSession()
	if err != nil {
		return errors.Wrap(err, "problem getting session")
	}
	defer session.Close()

	return errors.WithStack(db.C(coll).Remove(bson.M{"_id": id}))

}

// Count run a count command with the specified query against the collection.f
func Count(collection string, query interface{}) (int, error) {
	session, db, err := sink.GetMgoSession()

	if err != nil {
		return 0, err
	}
	defer session.Close()

	return db.C(collection).Find(query).Count()
}
