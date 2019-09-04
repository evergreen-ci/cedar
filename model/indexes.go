// +build ignore

package model

import (
	"github.com/mongodb/anser/bsonutil"
	"go.mongodb.org/mongo-driver/bson"
)

// SystemIndexes holds the keys, options and the collection for an index.
// See
// https://docs.mongodb.com/manual/reference/method/db.collection.createIndex
// for more info.
type SystemIndexes struct {
	Keys       bson.D
	Options    bson.D
	Collection string
}

// GetRequiredIndexes returns required indexes for the Cedar database.
func GetRequiredIndexes() []SystemIndexes {
	return []SystemIndexes{
		{
			Keys:       bson.D{{perfCreatedAtKey, 1}, {perfCompletedAtKey, 1}},
			Collection: perfResultCollection,
		},
		{
			Keys:       bson.D{{bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoProjectKey), 1}},
			Collection: perfResultCollection,
		},
		{
			Keys:       bson.D{{bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVersionKey), 1}},
			Collection: perfResultCollection,
		},
		{
			Keys:       bson.D{{bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVariantKey), 1}},
			Collection: perfResultCollection,
		},
		{
			Keys:       bson.D{{bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskNameKey), 1}},
			Collection: perfResultCollection,
		},
		{
			Keys:       bson.D{{bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskIDKey), 1}},
			Collection: perfResultCollection,
		},
		{
			Keys:       bson.D{{bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoExecutionKey), 1}},
			Collection: perfResultCollection,
		},
		{
			Keys:       bson.D{{bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTestNameKey), 1}},
			Collection: perfResultCollection,
		},
		{
			Keys:       bson.D{{bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTrialKey), 1}},
			Collection: perfResultCollection,
		},
		{
			Keys:       bson.D{{bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoParentKey), 1}},
			Collection: perfResultCollection,
		},
		{
			Keys:       bson.D{{bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTagsKey), 1}},
			Collection: perfResultCollection,
		},
		{
			Keys:       bson.D{{bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoSchemaKey), 1}},
			Collection: perfResultCollection,
		},
		{
			Keys:       bson.D{{bsonutil.GetDottedKeyName(dbUserLoginCacheKey, loginCacheTokenKey), 1}},
			Options:    bson.D{{"unique": true}},
			Collection: userCollection,
		},
		{
			Keys:       bson.D{{dbUserAPIKeyKey, 1}},
			Collection: userCollection,
		},
		{
			Keys:       bson.D{{logCreatedAtKey, 1}},
			Collection: buildloggerCollection,
		},
		{
			Keys:       bson.D{{bsonutil.GetDottedKeyName(logInfoKey, logInfoTaskIDKey), 1}},
			Collection: buildloggerCollection,
		},
	}
}
