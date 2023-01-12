package model

import (
	"context"
	"fmt"
	"strings"

	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
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

// GetRequiredIndexes returns required indexes for the Cedar DB.
// IMPORTANT: this should be updated whenever an index is created.
func GetRequiredIndexes() []SystemIndexes {
	return []SystemIndexes{
		{
			Keys:       bson.D{{Key: bsonutil.GetDottedKeyName(dbUserLoginCacheKey, loginCacheTokenKey), Value: 1}},
			Options:    bson.D{{Key: "unique", Value: true}},
			Collection: userCollection,
		},
		{
			Keys:       bson.D{{Key: dbUserAPIKeyKey, Value: 1}},
			Collection: userCollection,
		},
		{
			Keys:       bson.D{{Key: perfCreatedAtKey, Value: 1}, {Key: perfCompletedAtKey, Value: 1}},
			Collection: perfResultCollection,
		},
		{
			Keys: bson.D{
				{Key: bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskIDKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTagsKey), Value: 1},
				{Key: perfCreatedAtKey, Value: 1},
			},
			Collection: perfResultCollection,
		},
		{
			Keys: bson.D{
				{Key: bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskIDKey), Value: 1},
				{Key: perfCreatedAtKey, Value: 1},
			},
			Collection: perfResultCollection,
		},
		{
			Keys: bson.D{
				{Key: bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVersionKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTagsKey), Value: 1},
				{Key: perfCreatedAtKey, Value: 1},
			},
			Collection: perfResultCollection,
		},
		{
			Keys: bson.D{
				{Key: bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVersionKey), Value: 1},
				{Key: perfCreatedAtKey, Value: 1},
			},
			Collection: perfResultCollection,
		},
		{
			Keys:       bson.D{{Key: bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoParentKey), Value: 1}},
			Collection: perfResultCollection,
		},
		{
			Keys: bson.D{
				{Key: bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoProjectKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVariantKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskNameKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTestNameKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoMainlineKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoOrderKey), Value: 1},
			},
			Collection: perfResultCollection,
		},
		{
			Keys: bson.D{
				{Key: bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoProjectKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVariantKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskNameKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoMainlineKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoOrderKey), Value: 1},
			},
			Collection: perfResultCollection,
		},
		{
			Keys: bson.D{
				{Key: bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVariantKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskNameKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoMainlineKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoOrderKey), Value: 1},
			},
			Collection: perfResultCollection,
		},
		{
			Keys: bson.D{
				{Key: bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskNameKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoMainlineKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoOrderKey), Value: 1},
			},
			Collection: perfResultCollection,
		},
		{
			Keys: bson.D{
				{Key: bsonutil.GetDottedKeyName(perfArtifactsKey, artifactInfoFormatKey), Value: 1},
				{Key: perfFailedRollupAttempts, Value: 1},
				{Key: bsonutil.GetDottedKeyName(perfRollupsKey, perfRollupsStatsKey, perfRollupValueNameKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(perfRollupsKey, perfRollupsStatsKey, perfRollupValueVersionKey), Value: 1},
				{Key: perfCreatedAtKey, Value: 1},
			},
			Collection: perfResultCollection,
		},
		{
			Keys: bson.D{
				{Key: bsonutil.GetDottedKeyName(logInfoKey, logInfoTaskIDKey), Value: 1},
				{Key: logCreatedAtKey, Value: -1},
			},
			Collection: buildloggerCollection,
		},
		{
			Keys: bson.D{
				{Key: bsonutil.GetDottedKeyName(logInfoKey, logInfoTaskIDKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(logInfoKey, logInfoExecutionKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(logInfoKey, logInfoProcessNameKey), Value: 1},
				{Key: logCreatedAtKey, Value: -1},
			},
			Collection: buildloggerCollection,
		},
		{
			Keys: bson.D{
				{Key: bsonutil.GetDottedKeyName(logInfoKey, logInfoTaskIDKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(logInfoKey, logInfoExecutionKey), Value: 1},
				{Key: logCreatedAtKey, Value: -1},
			},
			Collection: buildloggerCollection,
		},
		{
			Keys: bson.D{
				{Key: bsonutil.GetDottedKeyName(logInfoKey, logInfoTaskIDKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(logInfoKey, logInfoTestNameKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(logInfoKey, logInfoExecutionKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(logInfoKey, logInfoProcessNameKey), Value: 1},
				{Key: logCreatedAtKey, Value: -1},
			},
			Collection: buildloggerCollection,
		},
		{
			Keys: bson.D{
				{Key: bsonutil.GetDottedKeyName(logInfoKey, logInfoTaskIDKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(logInfoKey, logInfoTestNameKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(logInfoKey, logInfoExecutionKey), Value: 1},
				{Key: logCreatedAtKey, Value: -1},
			},
			Collection: buildloggerCollection,
		},
		{
			Keys: bson.D{
				{Key: bsonutil.GetDottedKeyName(systemMetricsInfoKey, systemMetricsInfoTaskIDKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(systemMetricsInfoKey, systemMetricsInfoExecutionKey), Value: 1},
			},
			Collection: systemMetricsCollection,
		},
		{
			Keys: bson.D{
				{Key: bsonutil.GetDottedKeyName(testResultsInfoKey, testResultsInfoTaskIDKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(testResultsInfoKey, testResultsInfoExecutionKey), Value: 1},
			},
			Collection: testResultsCollection,
		},
		{
			Keys: bson.D{
				{Key: bsonutil.GetDottedKeyName(testResultsInfoKey, testResultsInfoDisplayTaskIDKey), Value: 1},
				{Key: bsonutil.GetDottedKeyName(testResultsInfoKey, testResultsInfoExecutionKey), Value: 1},
			},
			Options: bson.D{
				{
					Key: "partialFilterExpression",
					Value: bson.M{
						bsonutil.GetDottedKeyName(testResultsInfoKey, testResultsInfoDisplayTaskIDKey): bson.M{"$exists": true},
					},
				},
			},
			Collection: testResultsCollection,
		},
		{
			Keys:       bson.D{{Key: dbUserAPIKeyKey, Value: 1}},
			Collection: userCollection,
		},
		{
			Keys:       bson.D{{Key: bsonutil.GetDottedKeyName(dbUserLoginCacheKey, loginCacheTokenKey), Value: 1}},
			Collection: userCollection,
		},
	}
}

// CheckIndexes checks that all the given indexes are in the DB. It does
// not check specifically for any index options (e.g. unique indexes).
func CheckIndexes(ctx context.Context, db *mongo.Database, indexes []SystemIndexes) error {
	found := map[string]bool{}
	for _, index := range indexes {
		found[docToString(index.Keys)] = false
	}

	catcher := grip.NewBasicCatcher()
	indexesByColl := map[string][]dbIndex{}
	for _, index := range indexes {
		if _, ok := indexesByColl[index.Collection]; ok {
			continue
		}

		collIndexes, err := getCollIndexes(ctx, db, index.Collection)
		if err != nil {
			catcher.Wrapf(err, "getting indexes for collection %s", index.Collection)
			continue
		}
		indexesByColl[index.Collection] = collIndexes

		for _, collIndex := range collIndexes {
			docStr := docToString(collIndex.Key)
			if _, ok := found[docStr]; !ok {
				// Ignore extra indexes.
				continue
			}
			found[docStr] = true
		}
	}

	for _, index := range indexes {
		if !found[docToString(index.Keys)] {
			catcher.Errorf("expected index '%v' is missing from collection '%s'", index.Keys, index.Collection)
		}
	}

	return catcher.Resolve()
}

func docToString(doc bson.D) string {
	var s string
	for _, elem := range doc {
		s = strings.Join([]string{s, fmt.Sprintf("%s_%v", elem.Key, elem.Value)}, "_")
	}
	return s
}

type dbIndex struct {
	Key bson.D `bson:"key"`
}

func getCollIndexes(ctx context.Context, db *mongo.Database, collName string) ([]dbIndex, error) {
	cursor, err := db.Collection(collName).Indexes().List(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "listing indexes for collection '%s'", collName)
	}
	defer cursor.Close(ctx)

	var collIndexes []dbIndex
	if err := cursor.All(ctx, &collIndexes); err != nil {
		return nil, errors.Wrapf(err, "getting all indexes for collection '%s'", collName)
	}
	return collIndexes, nil
}
