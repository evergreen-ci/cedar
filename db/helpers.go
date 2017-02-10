package db

import "gopkg.in/mgo.v2/bson"

func StringKeyQuery(keyName, value string) *Q {
	return Query(bson.M{keyName: value})
}

func IDQuery(value interface{}) *Q {
	return Query(bson.M{"_id": value})
}
