package model

import (
	"bytes"
	"context"
	"crypto/sha1"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/pail"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
)

const testResultsCollection = "test_results"

// TestResults describes metadata for a task execution and its test results.
type TestResults struct {
	ID          string                  `bson:"_id,omitempty"`
	Info        TestResultsInfo         `bson:"info"`
	CreatedAt   time.Time               `bson:"created_at"`
	CompletedAt time.Time               `bson:"completed_at"`
	Artifact    TestResultsArtifactInfo `bson:"artifact"`

	env       cedar.Environment
	populated bool
}

var (
	testResultsIDKey          = bsonutil.MustHaveTag(TestResults{}, "ID")
	testResultsInfoKey        = bsonutil.MustHaveTag(TestResults{}, "Info")
	testResultsCreatedAtKey   = bsonutil.MustHaveTag(TestResults{}, "CreatedAt")
	testResultsCompletedAtKey = bsonutil.MustHaveTag(TestResults{}, "CompletedAt")
	testResultsArtifactKey    = bsonutil.MustHaveTag(TestResults{}, "Artifact")
)

// CreateTestResults is an entry point for creating a new TestResults.
func CreateTestResults(info TestResultsInfo, artifactStorageType PailType) *TestResults {
	return &TestResults{
		ID:        info.ID(),
		Info:      info,
		CreatedAt: time.Now(),
		Artifact: TestResultsArtifactInfo{
			Type:    artifactStorageType,
			Prefix:  info.ID(),
			Version: 0,
		},
		populated: true,
	}
}

// Setup sets the environment. The environment is required for numerous
// functions on TestResults.
func (t *TestResults) Setup(e cedar.Environment) { t.env = e }

// IsNil returns if the TestResults is populated or not.
func (t *TestResults) IsNil() bool { return !t.populated }

// Find searches the database for the TestResults. The enviromemt should not be
// nil.
func (t *TestResults) Find(ctx context.Context) error {
	if t.env == nil {
		return errors.New("cannot find with a nil environment")
	}

	if t.ID == "" {
		t.ID = t.Info.ID()
	}

	t.populated = false
	err := t.env.GetDB().Collection(testResultsCollection).FindOne(ctx, bson.M{"_id": t.ID}).Decode(t)
	if db.ResultsNotFound(err) {
		return errors.Wrapf(err, "could not find test results record with id %s in the database", t.ID)
	} else if err != nil {
		return errors.Wrap(err, "problem finding test results record")
	}
	t.populated = true

	return nil
}

// SaveNew saves a new TestResults to the database, if a document with the same
// ID already exists an error is returned. The TestResults should be populated
// and the environment should not be nil.
func (t *TestResults) SaveNew(ctx context.Context) error {
	if !t.populated {
		return errors.New("cannot save unpopulated test results")
	}
	if t.env == nil {
		return errors.New("cannot save with a nil environment")
	}

	if t.ID == "" {
		t.ID = t.Info.ID()
	}

	insertResult, err := t.env.GetDB().Collection(testResultsCollection).InsertOne(ctx, t)
	grip.DebugWhen(err == nil, message.Fields{
		"collection":   testResultsCollection,
		"id":           t.ID,
		"insertResult": insertResult,
		"op":           "save new test results record",
	})

	return errors.Wrapf(err, "problem saving new test results record %s", t.ID)
}

// Remove removes the test results document from the database. The environment
// should not be nil.
func (t *TestResults) Remove(ctx context.Context) error {
	if t.env == nil {
		return errors.New("cannot remove with a nil environment")
	}

	if t.ID == "" {
		t.ID = t.Info.ID()
	}

	deleteResult, err := t.env.GetDB().Collection(testResultsCollection).DeleteOne(ctx, bson.M{"_id": t.ID})
	grip.DebugWhen(err == nil, message.Fields{
		"collection":   testResultsCollection,
		"id":           t.ID,
		"deleteResult": deleteResult,
		"op":           "remove test results record",
	})

	return errors.Wrapf(err, "problem removing test results record with _id %s", t.ID)
}

// Append uploads test results to the offline blob storage bucket configured
// for the task execution. The TestResults should be populated and the
// environment should not be nil.
func (t *TestResults) Append(ctx context.Context, results []TestResult) error {
	if !t.populated {
		return errors.New("cannot append with populated test results")
	}
	if t.env == nil {
		return errors.New("cannot not append test results with a nil environment")
	}
	if len(results) == 0 {
		grip.Warning(message.Fields{
			"collection": testResultsCollection,
			"id":         t.ID,
			"message":    "append called with no test results",
		})
		return nil
	}

	conf := &CedarConfig{}
	conf.Setup(t.env)
	if err := conf.Find(); err != nil {
		return errors.Wrap(err, "problem getting application configuration")
	}
	bucket, err := t.Artifact.Type.Create(
		ctx,
		t.env,
		conf.Bucket.TestResultsBucket,
		t.Artifact.Prefix,
		string(pail.S3PermissionsPrivate),
		true,
	)
	if err != nil {
		return errors.Wrap(err, "problem creating bucket")
	}

	for _, result := range results {
		data, err := bson.Marshal(result)
		if err != nil {
			return errors.Wrap(err, "problem marshalling bson")
		}

		if err := bucket.Put(ctx, result.TestName, bytes.NewReader(data)); err != nil {
			return errors.Wrap(err, "problem uploading test results to bucket")
		}

	}

	return nil
}

// Download returns a TestResult slice with the corresponding results stored in
// the offline blob storage. The TestResults should be populated and the
// environment should not be nil.
func (t *TestResults) Download(ctx context.Context) ([]TestResult, error) {
	if !t.populated {
		return nil, errors.New("cannot download with populated test results")
	}
	if t.env == nil {
		return nil, errors.New("cannot download test results with a nil environment")
	}

	if t.ID == "" {
		t.ID = t.Info.ID()
	}

	conf := &CedarConfig{}
	conf.Setup(t.env)
	if err := conf.Find(); err != nil {
		return nil, errors.Wrap(err, "problem getting application configuration")
	}

	bucket, err := t.Artifact.Type.Create(
		ctx,
		t.env,
		conf.Bucket.TestResultsBucket,
		t.Artifact.Prefix,
		string(pail.S3PermissionsPrivate),
		false,
	)
	if err != nil {
		return nil, errors.Wrap(err, "problem creating bucket")
	}

	iter, err := bucket.List(ctx, "")
	if err != nil {
		return nil, errors.Wrap(err, "problem listing bucket items")
	}

	results := []TestResult{}
	for iter.Next(ctx) {
		r, err := iter.Item().Get(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "problem getting test result")
		}
		defer func() {
			grip.Error(message.WrapError(r.Close(), message.Fields{
				"message":  "problem closing bucket reader",
				"bucket":   conf.Bucket.TestResultsBucket,
				"prefix":   t.Artifact.Prefix,
				"path":     iter.Item(),
				"location": t.Artifact.Type,
			}))
		}()

		data, err := ioutil.ReadAll(r)
		if err != nil {
			return nil, errors.Wrap(err, "problem reading test result data")
		}
		result := TestResult{}
		if err = bson.Unmarshal(data, &result); err != nil {
			return nil, errors.Wrap(err, "problem unmarshalling bson test result")
		}
		results = append(results, result)
	}

	return results, nil
}

// Close "closes out" by populating the completed_at field. The environment
// should not be nil.
func (t *TestResults) Close(ctx context.Context) error {
	if t.env == nil {
		return errors.New("cannot close log with a nil environment")
	}

	if t.ID == "" {
		t.ID = t.Info.ID()
	}

	completedAt := time.Now()
	updateResult, err := t.env.GetDB().Collection(testResultsCollection).UpdateOne(
		ctx,
		bson.M{"_id": t.ID},
		bson.M{
			"$set": bson.M{
				testResultsCompletedAtKey: completedAt,
			},
		},
	)
	grip.DebugWhen(err == nil, message.Fields{
		"collection":   testResultsCollection,
		"id":           t.ID,
		"completed_at": completedAt,
		"updateResult": updateResult,
		"op":           "close test results record",
	})
	if err == nil && updateResult.MatchedCount == 0 {
		err = errors.Errorf("could not find test results record with id %s in the database", t.ID)
	}

	return errors.Wrapf(err, "problem closing test result record with id %s", t.ID)
}

// TestResultsInfo describes information unique to a single task execution.
type TestResultsInfo struct {
	Project     string `bson:"project,omitempty"`
	Version     string `bson:"version,omitempty"`
	Variant     string `bson:"variant,omitempty"`
	TaskName    string `bson:"task_name,omitempty"`
	TaskID      string `bson:"task_id,omitempty"`
	Execution   int    `bson:"execution"`
	RequestType string `bson:"request_type,omitempty"`
	Mainline    bool   `bson:"mainline,omitempty"`
	Schema      int    `bson:"schema,omitempty"`
}

var (
	testResultsInfoProjectKey     = bsonutil.MustHaveTag(TestResultsInfo{}, "Project")
	testResultsInfoVersionKey     = bsonutil.MustHaveTag(TestResultsInfo{}, "Version")
	testResultsInfoVariantKey     = bsonutil.MustHaveTag(TestResultsInfo{}, "Variant")
	testResultsInfoTaskNameKey    = bsonutil.MustHaveTag(TestResultsInfo{}, "TaskName")
	testResultsInfoTaskIDKey      = bsonutil.MustHaveTag(TestResultsInfo{}, "TaskID")
	testResultsInfoExecutionKey   = bsonutil.MustHaveTag(TestResultsInfo{}, "Execution")
	testResultsInfoRequestTypeKey = bsonutil.MustHaveTag(TestResultsInfo{}, "RequestType")
	testResultsInfoMainlineKey    = bsonutil.MustHaveTag(TestResultsInfo{}, "Mainline")
	testResultsInfoSchemaKey      = bsonutil.MustHaveTag(TestResultsInfo{}, "Schema")
)

// ID creates a unique hash for a TestResults record.
func (id *TestResultsInfo) ID() string {
	var hash hash.Hash

	if id.Schema == 0 {
		hash = sha1.New()
		_, _ = io.WriteString(hash, id.Project)
		_, _ = io.WriteString(hash, id.Version)
		_, _ = io.WriteString(hash, id.Variant)
		_, _ = io.WriteString(hash, id.TaskName)
		_, _ = io.WriteString(hash, id.TaskID)
		_, _ = io.WriteString(hash, fmt.Sprint(id.Execution))
		_, _ = io.WriteString(hash, id.RequestType)
	} else {
		panic("unsupported schema")
	}

	return fmt.Sprintf("%x", hash.Sum(nil))
}

// TestResult describes a single test result to be stored as a BSON object in
// some type of pail bucket storage.
type TestResult struct {
	TestName       string    `bson:"test_name"`
	Trial          int       `bson:"trial"`
	Status         string    `bson:"status"`
	LogURL         string    `bson:"log_url"`
	LineNum        int       `bson:"line_num"`
	TaskCreateTime time.Time `bson:"task_create_time"`
	TestStartTime  time.Time `bson:"test_start_time"`
	TestEndTime    time.Time `bson:"test_end_time"`
}
