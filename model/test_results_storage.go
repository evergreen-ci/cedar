package model

import "github.com/mongodb/anser/bsonutil"

// TestResultsArtifactInfo describes a bucket of test results for a given task
// execution stored in some kind of offline storage. It is the bridge between
// pail-backed offline log storage and the cedar-based log metadata storage.
// The prefix field indicates the name of the "sub-bucket". The top level
// bucket is accesible via the cedar.Environment interface.
type TestResultsArtifactInfo struct {
	Type    PailType `bson:"type"`
	Prefix  string   `bson:"prefix"`
	Version int      `bson:"version"`
}

var (
	testResultsArtifactInfoTypeKey    = bsonutil.MustHaveTag(TestResultsArtifactInfo{}, "Type")
	testResultsArtifactInfoPrefixKey  = bsonutil.MustHaveTag(TestResultsArtifactInfo{}, "Prefix")
	testResultsArtifactInfoVersionKey = bsonutil.MustHaveTag(TestResultsArtifactInfo{}, "Version")
)
