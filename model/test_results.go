package model

import (
	"bytes"
	"context"
	"crypto/sha1"
	"fmt"
	"reflect"

	"hash"
	"io"
	"io/ioutil"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/pail"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/recovery"
	"github.com/pkg/errors"
	"github.com/xitongsys/parquet-go-source/buffer"
	"github.com/xitongsys/parquet-go/reader"
	"github.com/xitongsys/parquet-go/types"
	"github.com/xitongsys/parquet-go/writer"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (

	// FailedTestsSampleSize is the maximum size for the failed test
	// results sample.
	FailedTestsSampleSize = 10

	testResultsCollection = "test_results"
	parquetDateFormat     = "2006-01-02"
)

// invalidParquetAlertCount is a temporary count to avoid over-logging when we
// detect invalid Parquet test results.
//
// TODO (EVG-16140): Remove this var and related functions after we do the BSON
// to Parquet cutover.
var invalidParquetAlertCount = struct {
	mu    sync.RWMutex
	count int
}{}

func getInvalidParquetAlertCount() int {
	invalidParquetAlertCount.mu.RLock()
	defer invalidParquetAlertCount.mu.RUnlock()

	return invalidParquetAlertCount.count
}

func incInvalidParquetAlertCount() {
	invalidParquetAlertCount.mu.Lock()
	defer invalidParquetAlertCount.mu.Unlock()

	invalidParquetAlertCount.count++
}

// TestResults describes metadata for a task execution and its test results.
type TestResults struct {
	ID          string                  `bson:"_id,omitempty"`
	Info        TestResultsInfo         `bson:"info"`
	CreatedAt   time.Time               `bson:"created_at"`
	CompletedAt time.Time               `bson:"completed_at"`
	Artifact    TestResultsArtifactInfo `bson:"artifact"`
	Stats       TestResultsStats        `bson:"stats"`
	// FailedTestsSample is the first X failing tests of the test results.
	// This is an optimization for Evergreen's UI features that display a
	// limited number of failing tests for a task.
	FailedTestsSample []string `bson:"failed_tests_sample"`
	// Migration holds statistics for the BSON to Parquet backfill, it is
	// only present in the temporary test results migration collection.
	//
	// TODO (EVG-16140): Remove once we do the  BSON to Parquet cutover.
	Migration *MigrationStats `bson:"migration,omitempty"`

	env                cedar.Environment
	bucket             string
	prestoBucket       string
	prestoBucketPrefix string
	populated          bool
}

var (
	testResultsIDKey                = bsonutil.MustHaveTag(TestResults{}, "ID")
	testResultsInfoKey              = bsonutil.MustHaveTag(TestResults{}, "Info")
	testResultsCreatedAtKey         = bsonutil.MustHaveTag(TestResults{}, "CreatedAt")
	testResultsCompletedAtKey       = bsonutil.MustHaveTag(TestResults{}, "CompletedAt")
	testResultsArtifactKey          = bsonutil.MustHaveTag(TestResults{}, "Artifact")
	testResultsStatsKey             = bsonutil.MustHaveTag(TestResults{}, "Stats")
	testResultsFailedTestsSampleKey = bsonutil.MustHaveTag(TestResults{}, "FailedTestsSample")
	// TODO (EVG-16140): Remove once we do the BSON to Parquet cutover.
	TestResultsMigrationKey = bsonutil.MustHaveTag(TestResults{}, "Migration")
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

// PrestoPartitionKey returns the partition key for the S3 bucket in Presto.
func (t *TestResults) PrestoPartitionKey() string {
	return fmt.Sprintf("task_create_iso=%s/project=%s/%s", t.CreatedAt.UTC().Format(parquetDateFormat), t.Info.Project, t.Artifact.Prefix)
}

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

	allResults, err := t.downloadBSON(ctx)
	if err != nil && !pail.IsKeyNotFoundError(err) {
		return errors.Wrap(err, "getting uploaded test results")
	}
	allResults = append(allResults, results...)

	// TODO (EVG-16140): Stop calling this once we do the BSON to Parquet
	// cutover.
	if err := t.uploadBSON(ctx, allResults); err != nil {
		return errors.Wrap(err, "appending BSON test results")
	}

	if err = t.uploadParquet(ctx, t.convertToParquet(allResults)); err != nil {
		return errors.Wrap(err, "appending Parquet test results")
	}

	if err = t.env.GetStatsCache(cedar.StatsCacheTestResults).AddStat(cedar.Stat{
		Count:   len(results),
		Project: t.Info.Project,
		Version: t.Info.Version,
		TaskID:  t.Info.TaskID,
	}); err != nil {
		grip.Error(message.WrapError(err, message.Fields{
			"message": "stats were dropped",
			"cache":   cedar.StatsCacheTestResults,
		}))
	}

	return t.updateStatsAndFailedSample(ctx, results)
}

// TODO (EVG-16140): Remove this once we do the BSON to Parquet cutover.
func (t *TestResults) uploadBSON(ctx context.Context, results []TestResult) error {
	bucket, err := t.GetBucket(ctx)
	if err != nil {
		return err
	}

	data, err := bson.Marshal(&testResultsDoc{Results: results})
	if err != nil {
		return errors.Wrap(err, "marshalling BSON test results")
	}

	return errors.Wrap(bucket.Put(ctx, testResultsCollection, bytes.NewReader(data)), "uploading BSON test results")
}

func (t *TestResults) uploadParquet(ctx context.Context, results *ParquetTestResults) error {
	bucket, err := t.GetPrestoBucket(ctx)
	if err != nil {
		return err
	}
	w, err := bucket.Writer(ctx, t.PrestoPartitionKey())
	if err != nil {
		return errors.Wrap(err, "creating Presto bucket writer")
	}
	var closedWriter bool
	defer func() {
		if !closedWriter {
			_ = w.Close()
		}
	}()
	pw, err := writer.NewParquetWriterFromWriter(w, new(ParquetTestResults), 1)
	if err != nil {
		return errors.Wrap(err, "creating new Parquet writer")
	}
	if err = pw.Write(results); err != nil {
		return errors.Wrap(err, "writing Parquet test results")
	}
	if err = pw.WriteStop(); err != nil {
		return errors.Wrap(err, "stopping Parquet writer")
	}

	closedWriter = true
	return errors.Wrap(w.Close(), "closing Presto bucket writer")
}

func (t *TestResults) updateStatsAndFailedSample(ctx context.Context, results []TestResult) error {
	var failedCount int
	for i := 0; i < len(results); i++ {
		if strings.Contains(strings.ToLower(results[i].Status), "fail") {
			if len(t.FailedTestsSample) < FailedTestsSampleSize {
				t.FailedTestsSample = append(t.FailedTestsSample, results[i].GetDisplayName())
			}
			failedCount++
		}
	}

	updateResult, err := t.env.GetDB().Collection(testResultsCollection).UpdateOne(
		ctx,
		bson.M{testResultsIDKey: t.ID},
		bson.M{
			"$inc": bson.M{
				bsonutil.GetDottedKeyName(testResultsStatsKey, testResultsStatsTotalCountKey):  len(results),
				bsonutil.GetDottedKeyName(testResultsStatsKey, testResultsStatsFailedCountKey): failedCount,
			},
			"$set": bson.M{
				testResultsFailedTestsSampleKey: t.FailedTestsSample,
			},
		},
	)
	grip.DebugWhen(err == nil, message.Fields{
		"collection":          testResultsCollection,
		"id":                  t.ID,
		"inc_total_count":     len(results),
		"inc_failed_count":    failedCount,
		"failed_tests_sample": t.FailedTestsSample,
		"update_result":       updateResult,
		"op":                  "updating stats and failing tests sample",
	})
	if err == nil && updateResult.MatchedCount == 0 {
		err = errors.Errorf("could not find test results record with id %s in the database", t.ID)
	}

	t.Stats.TotalCount += len(results)
	t.Stats.FailedCount += failedCount

	return errors.Wrapf(err, "appending to failing tests sample for test result record with id %s", t.ID)
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

	switch t.Artifact.Version {
	case 0:
		bucket, err := t.GetBucket(ctx)
		if err != nil {
			return nil, err
		}

		var results []TestResult
		iter := NewTestResultsIterator(bucket)
		for iter.Next(ctx) {
			results = append(results, iter.Item())
		}

		catcher := grip.NewBasicCatcher()
		catcher.Wrap(iter.Err(), "iterating test results")
		catcher.Wrap(iter.Close(), "closing test results iterator")

		return results, catcher.Resolve()
	case 1:
		bsonResults, err := t.downloadBSON(ctx)
		if err != nil {
			return nil, err
		}
		parquetResults, err := t.downloadParquet(ctx)
		if err != nil && !pail.IsKeyNotFoundError(err) {
			return nil, err
		} else if err == nil && getInvalidParquetAlertCount() < 5 {
			if !reflect.DeepEqual(parquetResults, bsonResults) {
				grip.Warning(message.Fields{
					"message":   "BSON and Parquet test results differ",
					"task_id":   t.Info.TaskID,
					"execution": t.Info.Execution,
				})
				incInvalidParquetAlertCount()
			}
		}

		return bsonResults, nil
	default:
		return nil, errors.Errorf("unsupported test results artifact version '%d'", t.Artifact.Version)
	}
}

// TODO (EVG-16140): Move this to Download (above) once we do the BSON to
// Parquet cutover.
func (t *TestResults) downloadParquet(ctx context.Context) ([]TestResult, error) {
	prestoBucket, err := t.GetPrestoBucket(ctx)
	if err != nil {
		return nil, err
	}

	r, err := prestoBucket.Get(ctx, t.PrestoPartitionKey())
	if err != nil {
		return nil, errors.Wrap(err, "getting Parquet test results")
	}

	catcher := grip.NewBasicCatcher()
	data, err := ioutil.ReadAll(r)
	catcher.Wrap(err, "reading Parquet test results")
	catcher.Wrap(r.Close(), "closing Presto bucket reader")
	if catcher.HasErrors() {
		return nil, catcher.Resolve()
	}

	pr, err := reader.NewParquetReader(buffer.NewBufferFileFromBytes(data), new(ParquetTestResults), 1)
	if err != nil {
		return nil, errors.Wrap(err, "creating Parquet reader")
	}

	parquetResults := make([]ParquetTestResults, pr.GetNumRows())
	if err := pr.Read(&parquetResults); err != nil {
		return nil, errors.Wrap(err, "unmarshalling Parquet test results")
	}
	pr.ReadStop()

	var results []TestResult
	for _, result := range parquetResults {
		results = append(results, result.convertToTestResultSlice()...)
	}

	return results, nil
}

// TODO (EVG-16140): Remove this function once we do the BSON to Parquet
// cutover.
func (t *TestResults) downloadBSON(ctx context.Context) ([]TestResult, error) {
	bucket, err := t.GetBucket(ctx)
	if err != nil {
		return nil, err
	}

	r, err := bucket.Get(ctx, testResultsCollection)
	if err != nil {
		return nil, errors.Wrap(err, "getting BSON test results")
	}

	data, err := ioutil.ReadAll(r)
	catcher := grip.NewBasicCatcher()
	catcher.Wrap(err, "reading BSON test results")
	catcher.Wrap(r.Close(), "closing BSON test results bucket reader")
	if err := catcher.Resolve(); err != nil {
		return nil, err
	}

	var resultsDoc testResultsDoc
	if err = bson.Unmarshal(data, &resultsDoc); err != nil {
		return nil, errors.Wrap(err, "unmarshalling BSON test results")
	}

	return resultsDoc.Results, nil
}

// DownloadAndConvertToParquet returns all of the test results stored in
// offline blob storage converted to a ParquetTestResults struct for writing to
// Apache Parquet format. The TestResults should be populated and the
// environment should not be nil.
//
// TODO (EVG-16140): Remove this function once we do the BSON to Parquet
// cutover.
func (t *TestResults) DownloadAndConvertToParquet(ctx context.Context) (*ParquetTestResults, error) {
	results, err := t.Download(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "downloading BSON test results")
	}

	return t.convertToParquet(results), nil
}

// DownloadConvertAndWriteParquet downloads existing BSON test results,
// converts them to Apache Parquet format, and writes them to the Presto
// bucket.
//
// TODO (EVG-16140): Remove this function once we do the BSON to Parquet
// cutover.
func (t *TestResults) DownloadConvertAndWriteParquet(ctx context.Context) error {
	results, err := t.DownloadAndConvertToParquet(ctx)
	if err != nil {
		return err
	}

	return t.uploadParquet(ctx, results)
}

func (t *TestResults) convertToParquet(results []TestResult) *ParquetTestResults {
	convertedResults := make([]ParquetTestResult, len(results))
	for i, result := range results {
		convertedResults[i] = result.convertToParquet()
	}

	return &ParquetTestResults{
		TaskID:    t.Info.TaskID,
		Execution: int32(t.Info.Execution),
		Variant:   t.Info.Variant,
		Version:   t.Info.Version,
		CreatedAt: types.TimeToTIMESTAMP_MILLIS(t.CreatedAt.UTC(), true),
		Results:   convertedResults,
	}
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
		"collection":    testResultsCollection,
		"id":            t.ID,
		"completed_at":  completedAt,
		"update_result": updateResult,
		"op":            "close test results record",
	})
	if err == nil && updateResult.MatchedCount == 0 {
		err = errors.Errorf("could not find test results record with id %s in the database", t.ID)
	}

	return errors.Wrapf(err, "problem closing test result record with id %s", t.ID)
}

// GetBucket returns a bucket of all test results specified by the TestResults
// metadata object it's called on. The environment should not be nil.
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

// GetPrestoBucket returns an S3 bucket of all test results specified by the
// TestResults metadata object it's called on to be used with Presto. The
// environment should not be nil.
//
// TODO (EVG-16140): Remove this and use GetBucket (above) once we do the BSON
// to Parquet cutover.
func (t *TestResults) GetPrestoBucket(ctx context.Context) (pail.Bucket, error) {
	if t.prestoBucket == "" {
		conf := &CedarConfig{}
		conf.Setup(t.env)
		if err := conf.Find(); err != nil {
			return nil, errors.Wrap(err, "getting application configuration")
		}

		t.prestoBucket = conf.Bucket.PrestoBucket
		t.prestoBucketPrefix = conf.Bucket.PrestoTestResultsPrefix
	}

	bucket, err := t.Artifact.Type.Create(
		ctx,
		t.env,
		t.prestoBucket,
		t.prestoBucketPrefix,
		string(pail.S3PermissionsPrivate),
		false,
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

// TestResultsStats describes basic stats of the test results.
type TestResultsStats struct {
	TotalCount  int `bson:"total_count"`
	FailedCount int `bson:"failed_count"`
}

// TestResultsSample contains test names culled from a test result's FailedTestsSample.
type TestResultsSample struct {
	TaskID                  string
	Execution               int
	MatchingFailedTestNames []string
	TotalFailedTestNames    int
}

var (
	testResultsStatsTotalCountKey  = bsonutil.MustHaveTag(TestResultsStats{}, "TotalCount")
	testResultsStatsFailedCountKey = bsonutil.MustHaveTag(TestResultsStats{}, "FailedCount")
)

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
	BaseStatus      string    `bson:"-"`
	LogTestName     string    `bson:"log_test_name,omitempty"`
	LogURL          string    `bson:"log_url,omitempty"`
	RawLogURL       string    `bson:"raw_log_url,omitempty"`
	LineNum         int       `bson:"line_num"`
	TaskCreateTime  time.Time `bson:"task_create_time"`
	TestStartTime   time.Time `bson:"test_start_time"`
	TestEndTime     time.Time `bson:"test_end_time"`
}

// GetDisplayName returns the human-readable name of the test.
func (t TestResult) GetDisplayName() string {
	if t.DisplayTestName != "" {
		return t.DisplayTestName
	}
	return t.TestName
}

func (t TestResult) getDuration() time.Duration {
	return t.TestEndTime.Sub(t.TestStartTime)
}

func (t TestResult) convertToParquet() ParquetTestResult {
	return ParquetTestResult{
		TestName:        t.TestName,
		DisplayTestName: utility.ToStringPtr(t.DisplayTestName),
		GroupID:         utility.ToStringPtr(t.GroupID),
		Trial:           utility.ToInt32Ptr(int32(t.Trial)),
		Status:          t.Status,
		LogTestName:     utility.ToStringPtr(t.LogTestName),
		LogURL:          utility.ToStringPtr(t.LogURL),
		RawLogURL:       utility.ToStringPtr(t.RawLogURL),
		LineNum:         utility.ToInt32Ptr(int32(t.LineNum)),
		TaskCreateTime:  utility.ToInt64Ptr(types.TimeToTIMESTAMP_MILLIS(t.TaskCreateTime.UTC(), true)),
		TestStartTime:   utility.ToInt64Ptr(types.TimeToTIMESTAMP_MILLIS(t.TestStartTime.UTC(), true)),
		TestEndTime:     utility.ToInt64Ptr(types.TimeToTIMESTAMP_MILLIS(t.TestEndTime.UTC(), true)),
	}
}

type testResultsDoc struct {
	Results []TestResult `bson:"results"`
}

// FindTestResultsOptions allow for querying for test results with or without
// an execution value.
type FindTestResultsOptions struct {
	TaskID      string
	Execution   *int
	DisplayTask bool
}

func (opts *FindTestResultsOptions) validate() error {
	catcher := grip.NewBasicCatcher()

	catcher.NewWhen(opts.TaskID == "", "must specify a task ID")
	catcher.NewWhen(utility.FromIntPtr(opts.Execution) < 0, "cannot specify a negative execution number")

	return catcher.Resolve()
}

func (opts *FindTestResultsOptions) createFindOptions() *options.FindOptions {
	findOpts := options.Find().SetSort(bson.D{{Key: bsonutil.GetDottedKeyName(testResultsInfoKey, testResultsInfoExecutionKey), Value: -1}})
	if !opts.DisplayTask {
		findOpts = findOpts.SetLimit(1)
	}

	return findOpts
}

func (opts *FindTestResultsOptions) createFindQuery() map[string]interface{} {
	search := bson.M{}
	if opts.DisplayTask {
		search[bsonutil.GetDottedKeyName(testResultsInfoKey, testResultsInfoDisplayTaskIDKey)] = opts.TaskID
	} else {
		search[bsonutil.GetDottedKeyName(testResultsInfoKey, testResultsInfoTaskIDKey)] = opts.TaskID
	}
	if opts.Execution != nil {
		search[bsonutil.GetDottedKeyName(testResultsInfoKey, testResultsInfoExecutionKey)] = *opts.Execution
	}

	return search
}

func (opts *FindTestResultsOptions) createErrorMessage() string {
	var msg string
	if opts.DisplayTask {
		msg = fmt.Sprintf("could not find test results records with display_task_id %s", opts.TaskID)
	} else {
		msg = fmt.Sprintf("could not find test results record with task_id %s", opts.TaskID)
	}
	if opts.Execution != nil {
		msg += fmt.Sprintf(" and execution %d", opts.Execution)
	}
	msg += " in the database"

	return msg
}

// FindTestResults searches the database for the TestResults associated with
// the provided options. The environment should not be nil. If execution is
// nil, it will default to the most recent execution.
func FindTestResults(ctx context.Context, env cedar.Environment, opts FindTestResultsOptions) ([]TestResults, error) {
	if env == nil {
		return nil, errors.New("cannot find with a nil environment")
	}

	if err := opts.validate(); err != nil {
		return nil, errors.Wrap(err, "invalid find options")
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

// FindTestSamplesOptions specifies test results to collect test names from
// and regex filters to apply to the test names.
type FindTestSamplesOptions struct {
	Tasks           []FindTestResultsOptions
	TestNameRegexes []string
}

func (opts *FindTestSamplesOptions) createFindQuery() map[string]interface{} {
	findQueries := []bson.M{}
	for _, task := range opts.Tasks {
		findQueries = append(findQueries, task.createFindQuery())
	}

	return bson.M{"$or": findQueries}
}

func (opts *FindTestSamplesOptions) createFindOptions() *options.FindOptions {
	return options.Find().SetProjection(bson.M{
		bsonutil.GetDottedKeyName(testResultsInfoKey, testResultsInfoTaskIDKey):        1,
		bsonutil.GetDottedKeyName(testResultsInfoKey, testResultsInfoDisplayTaskIDKey): 1,
		bsonutil.GetDottedKeyName(testResultsInfoKey, testResultsInfoExecutionKey):     1,
		testResultsFailedTestsSampleKey:                                                1,
	})
}

func (opts *FindTestSamplesOptions) makeTestSamples(testResults []TestResults) ([]TestResultsSample, error) {
	samples := opts.consolidateSamples(testResults)
	if len(opts.TestNameRegexes) == 0 {
		return samples, nil
	}

	return filterTestNames(samples, opts.TestNameRegexes)
}

func (opts *FindTestSamplesOptions) consolidateSamples(testResults []TestResults) []TestResultsSample {
	type taskExecutionPair struct {
		taskID    string
		execution int
	}
	displayTaskMap := make(map[taskExecutionPair][]string)
	executionTaskMap := make(map[taskExecutionPair][]string)
	for _, result := range testResults {
		if result.Info.DisplayTaskID != "" {
			dt := taskExecutionPair{
				taskID:    result.Info.DisplayTaskID,
				execution: result.Info.Execution,
			}
			displayTaskMap[dt] = append(displayTaskMap[dt], result.FailedTestsSample...)
		}
		et := taskExecutionPair{
			taskID:    result.Info.TaskID,
			execution: result.Info.Execution,
		}
		executionTaskMap[et] = result.FailedTestsSample
	}

	samples := make([]TestResultsSample, 0, len(opts.Tasks))
	for _, t := range opts.Tasks {
		pair := taskExecutionPair{
			taskID:    t.TaskID,
			execution: utility.FromIntPtr(t.Execution),
		}

		var names []string
		var ok bool
		if t.DisplayTask {
			names, ok = displayTaskMap[pair]
		} else {
			names, ok = executionTaskMap[pair]
		}
		if !ok {
			continue
		}

		samples = append(samples, TestResultsSample{
			TaskID:                  t.TaskID,
			Execution:               utility.FromIntPtr(t.Execution),
			MatchingFailedTestNames: names,
			TotalFailedTestNames:    len(names),
		})
	}

	return samples
}

func filterTestNames(samples []TestResultsSample, regexStrings []string) ([]TestResultsSample, error) {
	regexes := []*regexp.Regexp{}
	for _, regexString := range regexStrings {
		testNameRegex, err := regexp.Compile(regexString)
		if err != nil {
			return nil, errors.Wrap(err, "invalid regex")
		}
		regexes = append(regexes, testNameRegex)
	}

	filteredSamples := []TestResultsSample{}
	for _, sample := range samples {
		filteredNames := []string{}
		for _, regex := range regexes {
			for _, name := range sample.MatchingFailedTestNames {
				if regex.MatchString(name) {
					filteredNames = append(filteredNames, name)
				}
			}
		}
		sample.MatchingFailedTestNames = filteredNames
		filteredSamples = append(filteredSamples, sample)
	}

	return filteredSamples, nil
}

// GetTestResultsFilteredSamples finds the specified test results and returns the filtered samples.
// The environment should not be nil.
func GetTestResultsFilteredSamples(ctx context.Context, env cedar.Environment, opts FindTestSamplesOptions) ([]TestResultsSample, error) {
	if env == nil {
		return nil, errors.New("cannot find with a nil environment")
	}

	cur, err := env.GetDB().Collection(testResultsCollection).Find(ctx, opts.createFindQuery(), opts.createFindOptions())
	if err != nil {
		return nil, errors.Wrap(err, "finding test results record(s)")
	}
	results := []TestResults{}
	if err := cur.All(ctx, &results); err != nil {
		return nil, errors.Wrap(err, "decoding test results record(s)")
	}

	return opts.makeTestSamples(results)
}

// TestResultsSortBy describes the property by which to sort a set of test
// results.
type TestResultsSortBy string

const (
	TestResultsSortByStart      TestResultsSortBy = "start"
	TestResultsSortByDuration   TestResultsSortBy = "duration"
	TestResultsSortByTestName   TestResultsSortBy = "test_name"
	TestResultsSortByStatus     TestResultsSortBy = "status"
	TestResultsSortByBaseStatus TestResultsSortBy = "base_status"
)

func (s TestResultsSortBy) validate() error {
	switch s {
	case TestResultsSortByStart, TestResultsSortByDuration, TestResultsSortByTestName,
		TestResultsSortByStatus, TestResultsSortByBaseStatus:
		return nil
	default:
		return errors.Errorf("unrecognized test results sort by key '%s'", s)
	}
}

// FilterAndSortTestResultsOptions allow for filtering, sorting, and paginating
// a set of test results.
type FilterAndSortTestResultsOptions struct {
	TestName     string
	Statuses     []string
	GroupID      string
	SortBy       TestResultsSortBy
	SortOrderDSC bool
	Limit        int
	Page         int
	BaseResults  *FindTestResultsOptions

	testNameRegex *regexp.Regexp
	baseStatusMap map[string]string
}

func (o *FilterAndSortTestResultsOptions) validate() error {
	catcher := grip.NewBasicCatcher()

	catcher.AddWhen(o.SortBy != "", o.SortBy.validate())
	catcher.NewWhen(o.SortBy == TestResultsSortByBaseStatus && o.BaseResults == nil, "must specify base task ID when sorting by base status")
	catcher.NewWhen(o.Limit < 0, "limit cannot be negative")
	catcher.NewWhen(o.Page < 0, "page cannot be negative")
	catcher.NewWhen(o.Limit == 0 && o.Page > 0, "cannot specify a page without a limit")

	if o.TestName != "" {
		var err error
		o.testNameRegex, err = regexp.Compile(o.TestName)
		catcher.Wrapf(err, "compiling test name regex")
	}

	o.baseStatusMap = map[string]string{}

	return catcher.Resolve()
}

// FindAndDownloadTestResults allow for finding, downloading, filtering,
// sorting, and paginating test results.
type FindAndDownloadTestResultsOptions struct {
	Find          FindTestResultsOptions
	FilterAndSort *FilterAndSortTestResultsOptions
}

// FindAndDownloadTestResults searches the database for the TestResults
// associated with the provided options and returns the downloaded test
// results filtered, sorted, and paginated. The environment should not be nil.
// If execution is nil, it will default to the most recent execution.
func FindAndDownloadTestResults(ctx context.Context, env cedar.Environment, opts FindAndDownloadTestResultsOptions) ([]TestResult, int, error) {
	testResults, err := FindTestResults(ctx, env, opts.Find)
	if err != nil {
		return nil, 0, err
	}

	testResultsChan := make(chan TestResults, len(testResults))
	for i := range testResults {
		testResultsChan <- testResults[i]
	}
	close(testResultsChan)

	var (
		combinedResults []TestResult
		cwg             sync.WaitGroup
		pwg             sync.WaitGroup
	)
	combinedResultsChan := make(chan []TestResult, len(testResults))
	catcher := grip.NewBasicCatcher()
	cwg.Add(1)
	go func() {
		defer func() {
			catcher.Add(recovery.HandlePanicWithError(recover(), nil, "test results download consumer"))
			cwg.Done()
		}()

		for results := range combinedResultsChan {
			combinedResults = append(combinedResults, results...)
		}
	}()

	for i := 0; i < runtime.NumCPU(); i++ {
		pwg.Add(1)
		go func() {
			defer func() {
				catcher.Add(recovery.HandlePanicWithError(recover(), nil, "test results download producer"))
				pwg.Done()
			}()

			for trs := range testResultsChan {
				results, err := trs.Download(ctx)
				if err != nil {
					catcher.Add(err)
					return
				}

				select {
				case <-ctx.Done():
					catcher.Add(ctx.Err())
					return
				case combinedResultsChan <- results:
				}
			}
		}()
	}
	pwg.Wait()
	close(combinedResultsChan)
	cwg.Wait()

	if catcher.HasErrors() {
		return nil, 0, catcher.Resolve()
	}

	return filterAndSortTestResults(ctx, env, combinedResults, opts.FilterAndSort)
}

// filterAndSortCedarTestResults takes a slice of TestResult objects and
// returns a filtered sorted and paginated version of that slice.
func filterAndSortTestResults(ctx context.Context, env cedar.Environment, results []TestResult, opts *FilterAndSortTestResultsOptions) ([]TestResult, int, error) {
	if opts == nil {
		return results, len(results), nil
	}

	if err := opts.validate(); err != nil {
		return nil, 0, errors.Wrap(err, "validating filter and sort test results options")
	}

	if opts.BaseResults != nil {
		baseResults, _, err := FindAndDownloadTestResults(ctx, env, FindAndDownloadTestResultsOptions{Find: *opts.BaseResults})
		if err != nil {
			return nil, 0, errors.Wrap(err, "getting base test results")
		}
		for _, result := range baseResults {
			opts.baseStatusMap[result.GetDisplayName()] = result.Status
		}
	}

	results = filterTestResults(results, opts)
	sortTestResults(results, opts)

	totalCount := len(results)
	if opts.Limit > 0 {
		offset := opts.Limit * opts.Page
		end := offset + opts.Limit
		if offset > totalCount {
			offset = totalCount
		}
		if end > totalCount {
			end = totalCount
		}
		results = results[offset:end]
	}

	if len(opts.baseStatusMap) > 0 {
		for i := range results {
			results[i].BaseStatus = opts.baseStatusMap[results[i].GetDisplayName()]
		}
	}

	return results, totalCount, nil
}

func filterTestResults(results []TestResult, opts *FilterAndSortTestResultsOptions) []TestResult {
	if opts.testNameRegex == nil && len(opts.Statuses) == 0 && opts.GroupID == "" {
		return results
	}

	var filteredResults []TestResult
	for _, result := range results {
		if opts.testNameRegex != nil && !opts.testNameRegex.MatchString(result.GetDisplayName()) {
			continue
		}
		if len(opts.Statuses) > 0 && !utility.StringSliceContains(opts.Statuses, result.Status) {
			continue
		}
		if opts.GroupID != "" && opts.GroupID != result.GroupID {
			continue
		}

		filteredResults = append(filteredResults, result)
	}

	return filteredResults
}

func sortTestResults(results []TestResult, opts *FilterAndSortTestResultsOptions) {
	switch opts.SortBy {
	case TestResultsSortByStart:
		sort.SliceStable(results, func(i, j int) bool {
			if opts.SortOrderDSC {
				return results[i].TestStartTime.After(results[j].TestStartTime)
			}
			return results[i].TestStartTime.Before(results[j].TestStartTime)
		})
	case TestResultsSortByDuration:
		sort.SliceStable(results, func(i, j int) bool {
			if opts.SortOrderDSC {
				return results[i].getDuration() > results[j].getDuration()
			}
			return results[i].getDuration() < results[j].getDuration()
		})
	case TestResultsSortByTestName:
		sort.SliceStable(results, func(i, j int) bool {
			if opts.SortOrderDSC {
				return results[i].GetDisplayName() > results[j].GetDisplayName()
			}
			return results[i].GetDisplayName() < results[j].GetDisplayName()
		})
	case TestResultsSortByStatus:
		sort.SliceStable(results, func(i, j int) bool {
			if opts.SortOrderDSC {
				return results[i].Status > results[j].Status
			}
			return results[i].Status < results[j].Status
		})
	case TestResultsSortByBaseStatus:
		sort.SliceStable(results, func(i, j int) bool {
			if opts.SortOrderDSC {
				return opts.baseStatusMap[results[i].GetDisplayName()] > opts.baseStatusMap[results[j].GetDisplayName()]
			}
			return opts.baseStatusMap[results[i].GetDisplayName()] < opts.baseStatusMap[results[j].GetDisplayName()]
		})
	}
}

// GetTestResultsStats fetches basic stats for the test results associated with
// the provided options. The environment should not be nil. If execution is
// nil, it will default to the most recent execution.
func GetTestResultsStats(ctx context.Context, env cedar.Environment, opts FindTestResultsOptions) (TestResultsStats, error) {
	var stats TestResultsStats

	if !opts.DisplayTask {
		testResultsRecords, err := FindTestResults(ctx, env, opts)
		if err != nil {
			return stats, err
		}

		return testResultsRecords[0].Stats, nil
	}

	if env == nil {
		return stats, errors.New("cannot find with a nil environment")
	}
	if err := opts.validate(); err != nil {
		return stats, errors.Wrap(err, "invalid find options")
	}

	pipeline := []bson.M{
		{"$match": opts.createFindQuery()},
		{"$group": bson.M{
			"_id": "$" + bsonutil.GetDottedKeyName(testResultsInfoKey, testResultsInfoExecutionKey),
			testResultsStatsTotalCountKey: bson.M{
				"$sum": "$" + bsonutil.GetDottedKeyName(testResultsStatsKey, testResultsStatsTotalCountKey),
			},
			testResultsStatsFailedCountKey: bson.M{
				"$sum": "$" + bsonutil.GetDottedKeyName(testResultsStatsKey, testResultsStatsFailedCountKey),
			},
		}},
	}
	if opts.Execution == nil {
		pipeline = append(pipeline, bson.M{
			"$sort": bson.D{
				{Key: "_id", Value: -1},
			},
		})
	}
	pipeline = append(pipeline, bson.M{"$limit": 1})

	cur, err := env.GetDB().Collection(testResultsCollection).Aggregate(ctx, pipeline)
	if err != nil {
		return stats, errors.Wrap(err, "aggregating test results stats")
	}
	if !cur.Next(ctx) {
		return stats, errors.Wrap(mongo.ErrNoDocuments, opts.createErrorMessage())
	}

	return stats, errors.Wrap(cur.Decode(&stats), "decoding aggregated test results stats")
}

// FindTestResultsByProjectOptions represent the set of options for finding
// test results in the database by project name.
type FindTestResultsByProjectOptions struct {
	Projects []string
	StartAt  time.Time
	EndAt    time.Time
}

func (o FindTestResultsByProjectOptions) validate() error {
	catcher := grip.NewBasicCatcher()

	catcher.NewWhen(len(o.Projects) == 0, "must specify at least one project")
	catcher.NewWhen(o.StartAt.After(o.EndAt), "the start date cannot be greater than the end date")
	catcher.NewWhen(o.EndAt.IsZero(), "end date cannot be zero")

	return catcher.Resolve()
}

// FindTestResultsByProject returns all test results in the database with the
// given set of project names and within the given date interval.
func FindTestResultsByProject(ctx context.Context, env cedar.Environment, opts FindTestResultsByProjectOptions) ([]TestResults, error) {
	if err := opts.validate(); err != nil {
		return nil, errors.Wrap(err, "invalid options")
	}

	filter := bson.M{
		bsonutil.GetDottedKeyName(testResultsInfoKey, testResultsInfoProjectKey): bson.M{"$in": opts.Projects},
		testResultsCreatedAtKey: bson.M{
			"$gte": opts.StartAt,
			"$lt":  opts.EndAt,
		},
	}

	cur, err := env.GetDB().Collection(testResultsCollection).Find(ctx, filter)
	if err != nil {
		return nil, errors.Wrap(err, "finding test results")
	}

	var results []TestResults
	if err = cur.All(ctx, &results); err != nil {
		return nil, errors.Wrap(err, "decoding test results")
	}

	for i := range results {
		results[i].env = env
		results[i].populated = true
	}

	return results, nil
}

// ParquetTestResults describes a set of test results from a task execution to
// be stored in Apache Parquet format.
type ParquetTestResults struct {
	Version   string              `parquet:"name=version, type=BYTE_ARRAY, convertedtype=UTF8"`
	Variant   string              `parquet:"name=variant, type=BYTE_ARRAY, convertedtype=UTF8"`
	TaskID    string              `parquet:"name=task_id, type=BYTE_ARRAY, convertedtype=UTF8"`
	Execution int32               `parquet:"name=execution, type=INT32"`
	CreatedAt int64               `parquet:"name=created_at, type=INT64, logicaltype=TIMESTAMP, logicaltype.unit=MILLIS, logicaltype.isadjustedtoutc=true"`
	Results   []ParquetTestResult `parquet:"name=results, type=LIST"`
}

func (r ParquetTestResults) convertToTestResultSlice() []TestResult {
	results := make([]TestResult, len(r.Results))
	for i := range r.Results {
		results[i] = TestResult{
			TaskID:          r.TaskID,
			Execution:       int(r.Execution),
			TestName:        r.Results[i].TestName,
			DisplayTestName: utility.FromStringPtr(r.Results[i].DisplayTestName),
			GroupID:         utility.FromStringPtr(r.Results[i].GroupID),
			Trial:           int(utility.FromInt32Ptr(r.Results[i].Trial)),
			Status:          r.Results[i].Status,
			LogTestName:     utility.FromStringPtr(r.Results[i].LogTestName),
			LogURL:          utility.FromStringPtr(r.Results[i].LogURL),
			RawLogURL:       utility.FromStringPtr(r.Results[i].RawLogURL),
			LineNum:         int(utility.FromInt32Ptr(r.Results[i].LineNum)),
			TaskCreateTime:  time.Unix(0, utility.FromInt64Ptr(r.Results[i].TaskCreateTime)*1e6).UTC(),
			TestStartTime:   time.Unix(0, utility.FromInt64Ptr(r.Results[i].TestStartTime)*1e6).UTC(),
			TestEndTime:     time.Unix(0, utility.FromInt64Ptr(r.Results[i].TestEndTime)*1e6).UTC(),
		}
	}

	return results
}

// ParquetTestResult describes a single test result to be stored in Apache
// Parquet file format.
type ParquetTestResult struct {
	TestName        string  `parquet:"name=test_name, type=BYTE_ARRAY, convertedtype=UTF8"`
	DisplayTestName *string `parquet:"name=display_test_name, type=BYTE_ARRAY, convertedtype=UTF8"`
	GroupID         *string `parquet:"name=group_id, type=BYTE_ARRAY, convertedtype=UTF8"`
	Trial           *int32  `parquet:"name=trial, type=INT32"`
	Status          string  `parquet:"name=status, type=BYTE_ARRAY, convertedtype=UTF8"`
	LogTestName     *string `parquet:"name=log_test_name, type=BYTE_ARRAY, convertedtype=UTF8"`
	LogURL          *string `parquet:"name=log_url, type=BYTE_ARRAY, convertedtype=UTF8"`
	RawLogURL       *string `parquet:"name=raw_log_url, type=BYTE_ARRAY, convertedtype=UTF8"`
	LineNum         *int32  `parquet:"name=line_num, type=INT32"`
	TaskCreateTime  *int64  `parquet:"name=task_Create_time, type=INT64, logicaltype=TIMESTAMP, logicaltype.unit=MILLIS, logicaltype.isadjustedtoutc=true"`
	TestStartTime   *int64  `parquet:"name=test_start_time, type=INT64, logicaltype=TIMESTAMP, logicaltype.unit=MILLIS, logicaltype.isadjustedtoutc=true"`
	TestEndTime     *int64  `parquet:"name=test_end_time, type=INT64, logicaltype=TIMESTAMP, logicaltype.unit=MILLIS, logicaltype.isadjustedtoutc=true"`
}
