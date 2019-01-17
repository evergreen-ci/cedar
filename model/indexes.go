package model

// +build ignore

import (
	"github.com/mongodb/anser/bsonutil"
	"gopkg.in/mgo.v2/bson"
)

// Indexes for "perf_results" collection.
const (
	PerfResultsCreatedCompletedIndex = bson.D{{perfCreatedAtKey, 1}, {perfCompletedAtKey, 1}}
	PerfResultsProjectIndex          = bson.D{
		{bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoProjectKey), 1},
	}
	PerfResultsVersionIndex = bson.D{
		{bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVersionKey), 1},
	}
	PerfResultsVariantIndex = bson.D{
		{bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVariantKey), 1},
	}
	PerfResultsTaskNameIndex = bson.D{
		{bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskNameKey), 1},
	}
	PerfResultsTaskIDIndex = bson.D{
		{bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskIDKey), 1},
	}
	PerfResultsExecutionIndex = bson.D{
		{bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoExecutionKey), 1},
	}
	PerfResultsTestNameIndex = bson.D{
		{bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTestNameKey), 1},
	}
	PerfResultsTrialIndex = bson.D{
		{bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTrialKey), 1},
	}
	PerfResultsParentIndex = bson.D{
		{bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoParentKey), 1},
	}
	PerfResultsTagsIndex = bson.D{
		{bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTagsKey), 1},
	}
	PerfResultsSchemaIndex = bson.D{
		{bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoSchemaKey), 1},
	}
)

// Indexes for "users" collection.
const (
	UsersTokenIndex  = bson.D{{bsonutil.GetDottedKeyName(dbUserLoginCacheKey, loginCacheTokenKey), 1}}
	UsersAPIKeyIndex = bson.D{{dbUserAPIKeyKey, 1}}
)
