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
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
	bucket    string
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
			Version: 1,
		},
		populated: true,
	}
}

// Setup sets the environment. The environment is required for numerous
// functions on TestResults.
func (t *TestResults) Setup(e cedar.Environment) { t.env = e }

// IsNil returns if the TestResults is populated or not.
func (t *TestResults) IsNil() bool { return !t.populated }

// Find searches the database for the TestResults. The environment should not be
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
		return errors.New("cannot append without populated test results")
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

	bucket, err := t.GetBucket(ctx)
	if err != nil {
		return err
	}

	var allResults testResultsDoc
	r, err := bucket.Get(ctx, testResultsCollection)
	if err != nil && !pail.IsKeyNotFoundError(err) {
		return errors.Wrap(err, "getting uploaded test results")
	}
	if err == nil {
		catcher := grip.NewBasicCatcher()

		data, err := ioutil.ReadAll(r)
		catcher.Add(r.Close())
		if err != nil {
			catcher.Wrap(err, "reading uploaded test results")
			return catcher.Resolve()
		}

		if err = bson.Unmarshal(data, &allResults); err != nil {
			catcher.Wrap(err, "unmarshalling uploaded test results")
			return catcher.Resolve()
		}
	}
	allResults.Results = append(allResults.Results, results...)

	data, err := bson.Marshal(&allResults)
	if err != nil {
		return errors.Wrap(err, "marshalling test results")
	}

	return errors.Wrap(bucket.Put(ctx, testResultsCollection, bytes.NewReader(data)), "uploading test results")
}

// Download returns a TestResult slice with the corresponding results stored in
// the offline blob storage. The TestResults should be populated and the
// environment should not be nil.
func (t *TestResults) Download(ctx context.Context) ([]TestResult, error) {
	if !t.populated {
		return nil, errors.New("cannot download without populated test results")
	}
	if t.env == nil {
		return nil, errors.New("cannot download test results with a nil environment")
	}

	if t.ID == "" {
		t.ID = t.Info.ID()
	}

	var results testResultsDoc
	catcher := grip.NewBasicCatcher()
	bucket, err := t.GetBucket(ctx)
	if err != nil {
		return nil, err
	}

	switch t.Artifact.Version {
	case 0:
		iter := NewTestResultsIterator(bucket)
		for iter.Next(ctx) {
			results.Results = append(results.Results, iter.Item())
		}

		catcher.Wrap(iter.Err(), "iterating test results")
		catcher.Wrap(iter.Close(), "closing test results iterator")
	case 1:
		r, err := bucket.Get(ctx, testResultsCollection)
		if err != nil {
			catcher.Wrap(err, "getting test results")
			break
		}

		data, err := ioutil.ReadAll(r)
		catcher.Add(r.Close())
		if err != nil {
			catcher.Wrap(err, "reading test results")
			break
		}

		catcher.Wrap(bson.Unmarshal(data, &results), "unmarshalling test results")
	default:
		catcher.Errorf("unsupported test results artifact version '%d'", t.Artifact.Version)
	}

	return results.Results, catcher.Resolve()
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

// GetBucket returns a bucket of all test results specified by the TestResults metadata
// object it's called on. The environment should not be nil.
func (t *TestResults) GetBucket(ctx context.Context) (pail.Bucket, error) {
	if t.bucket == "" {
		conf := &CedarConfig{}
		conf.Setup(t.env)
		if err := conf.Find(); err != nil {
			return nil, errors.Wrap(err, "getting application configuration")
		}
		t.bucket = conf.Bucket.TestResultsBucket
	}

	bucket, err := t.Artifact.Type.Create(
		ctx,
		t.env,
		t.bucket,
		t.Artifact.Prefix,
		string(pail.S3PermissionsPrivate),
		true,
	)
	if err != nil {
		return nil, errors.Wrap(err, "creating bucket")
	}

	return bucket, nil
}

// TestResultsInfo describes information unique to a single task execution.
type TestResultsInfo struct {
	Project                string `bson:"project,omitempty"`
	Version                string `bson:"version,omitempty"`
	Variant                string `bson:"variant,omitempty"`
	TaskName               string `bson:"task_name,omitempty"`
	DisplayTaskName        string `bson:"display_task_name,omitempty"`
	TaskID                 string `bson:"task_id,omitempty"`
	DisplayTaskID          string `bson:"display_task_id,omitempty"`
	Execution              int    `bson:"execution"`
	RequestType            string `bson:"request_type,omitempty"`
	Mainline               bool   `bson:"mainline,omitempty"`
	HistoricalDataDisabled bool   `bson:"historical_data_disabled"`
	Schema                 int    `bson:"schema,omitempty"`
}

var (
	testResultsInfoProjectKey             = bsonutil.MustHaveTag(TestResultsInfo{}, "Project")
	testResultsInfoVersionKey             = bsonutil.MustHaveTag(TestResultsInfo{}, "Version")
	testResultsInfoVariantKey             = bsonutil.MustHaveTag(TestResultsInfo{}, "Variant")
	testResultsInfoTaskNameKey            = bsonutil.MustHaveTag(TestResultsInfo{}, "TaskName")
	testResultsInfoDisplayTaskNameKey     = bsonutil.MustHaveTag(TestResultsInfo{}, "DisplayTaskName")
	testResultsInfoTaskIDKey              = bsonutil.MustHaveTag(TestResultsInfo{}, "TaskID")
	testResultsInfoDisplayTaskIDKey       = bsonutil.MustHaveTag(TestResultsInfo{}, "DisplayTaskID")
	testResultsInfoExecutionKey           = bsonutil.MustHaveTag(TestResultsInfo{}, "Execution")
	testResultsInfoRequestTypeKey         = bsonutil.MustHaveTag(TestResultsInfo{}, "RequestType")
	testResultsInfoMainlineKey            = bsonutil.MustHaveTag(TestResultsInfo{}, "Mainline")
	testResultsInfoHistoricalDataDisabled = bsonutil.MustHaveTag(TestResultsInfo{}, "HistoricalDataDisabled")
	testResultsInfoSchemaKey              = bsonutil.MustHaveTag(TestResultsInfo{}, "Schema")
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
		_, _ = io.WriteString(hash, id.DisplayTaskName)
		_, _ = io.WriteString(hash, id.TaskID)
		_, _ = io.WriteString(hash, id.DisplayTaskID)
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
	TaskID          string    `bson:"task_id"`
	Execution       int       `bson:"execution"`
	TestName        string    `bson:"test_name"`
	DisplayTestName string    `bson:"display_test_name"`
	GroupID         string    `bson:"group_id,omitempty"`
	Trial           int       `bson:"trial,omitempty"`
	Status          string    `bson:"status"`
	LogTestName     string    `bson:"log_test_name,omitempty"`
	LogURL          string    `bson:"log_url,omitempty"`
	RawLogURL       string    `bson:"raw_log_url,omitempty"`
	LineNum         int       `bson:"line_num"`
	TaskCreateTime  time.Time `bson:"task_create_time"`
	TestStartTime   time.Time `bson:"test_start_time"`
	TestEndTime     time.Time `bson:"test_end_time"`
}

type testResultsDoc struct {
	Results []TestResult `bson:"results"`
}

// TestResultsFindOptions allows for querying for test results with or without
// an execution value.
type TestResultsFindOptions struct {
	TaskID         string
	DisplayTaskID  string
	Execution      int
	EmptyExecution bool
}

func (opts *TestResultsFindOptions) createFindOptions() *options.FindOptions {
	findOpts := options.Find().SetSort(bson.D{{Key: bsonutil.GetDottedKeyName(testResultsInfoKey, testResultsInfoExecutionKey), Value: -1}})
	if opts.TaskID != "" {
		findOpts = findOpts.SetLimit(1)
	}

	return findOpts
}

func (opts *TestResultsFindOptions) createFindQuery() map[string]interface{} {
	search := bson.M{}
	if opts.TaskID != "" {
		search[bsonutil.GetDottedKeyName(testResultsInfoKey, testResultsInfoTaskIDKey)] = opts.TaskID
	} else {
		search[bsonutil.GetDottedKeyName(testResultsInfoKey, testResultsInfoDisplayTaskIDKey)] = opts.DisplayTaskID
	}
	if !opts.EmptyExecution {
		search[bsonutil.GetDottedKeyName(testResultsInfoKey, testResultsInfoExecutionKey)] = opts.Execution
	}

	return search
}

func (opts *TestResultsFindOptions) createErrorMessage() string {
	var msg string
	if opts.TaskID != "" {
		msg = fmt.Sprintf("could not find test result record with task_id %s", opts.TaskID)
	} else {
		msg = fmt.Sprintf("could not find test results records with display_task_id %s", opts.DisplayTaskID)
	}
	if !opts.EmptyExecution {
		msg += fmt.Sprintf(" and execution %d", opts.Execution)
	}
	msg += " in the database"

	return msg
}

// FindTestResults searches the database for the TestResults associated with
// the provided options. The environment should not be nil. If execution is
// empty, it will default to the most recent execution.
func FindTestResults(ctx context.Context, env cedar.Environment, opts TestResultsFindOptions) ([]TestResults, error) {
	if env == nil {
		return nil, errors.New("cannot find with a nil environment")
	}

	if (opts.TaskID == "" && opts.DisplayTaskID == "") || (opts.TaskID != "" && opts.DisplayTaskID != "") {
		return nil, errors.New("must specify either task_id or display_task_id")
	}

	results := []TestResults{}
	cur, err := env.GetDB().Collection(testResultsCollection).Find(ctx, opts.createFindQuery(), opts.createFindOptions())
	if err != nil {
		return nil, errors.Wrap(err, "finding test results record(s)")
	}
	if err := cur.All(ctx, &results); err != nil {
		return nil, errors.Wrap(err, "decoding test results record(s)")
	}
	if len(results) == 0 {
		return nil, errors.Wrap(mongo.ErrNoDocuments, opts.createErrorMessage())
	}

	execution := results[0].Info.Execution
	for i := range results {
		if results[i].Info.Execution != execution {
			results = results[:i]
			break
		}
		results[i].populated = true
		results[i].env = env
	}

	return results, nil
}

// FindAndDownloadTestResults searches the database for the TestResults
// associated with the provided options and returns a TestResultsIterator with
// the downloaded test results. The environment should not be nil. If execution
// is empty it will default to the most recent execution.
func FindAndDownloadTestResults(ctx context.Context, env cedar.Environment, opts TestResultsFindOptions) ([]TestResult, error) {
	testResults, err := FindTestResults(ctx, env, opts)
	if err != nil {
		return nil, err
	}

	var combinedResults []TestResult
	for i := range testResults {
		results, err := testResults[i].Download(ctx)
		if err != nil {
			return nil, err
		}

		combinedResults = append(combinedResults, results...)
	}

	return combinedResults, nil
}
