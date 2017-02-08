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
