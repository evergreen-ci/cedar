package db

import "gopkg.in/mgo.v2/bson"

// StringKeyQuery returns a query object with a filter, where the
// specified key (by string), matches the specified string value.
func StringKeyQuery(keyName, value string) *Q {
	return Query(bson.M{keyName: value})
}

// IDQuery returns a query object with a filter for a document that
// matches the specified ID.
func IDQuery(value interface{}) *Q {
	return Query(bson.M{"_id": value})
}
