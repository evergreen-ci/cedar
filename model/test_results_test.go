package model

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/pail"
	"github.com/evergreen-ci/utility"
	goparquet "github.com/fraugster/parquet-go"
	"github.com/fraugster/parquet-go/floor"
	"github.com/mongodb/grip/sometimes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestCreateTestResults(t *testing.T) {
	expected := getTestResults()
	actual := CreateTestResults(expected.Info, PailLocal)
	assert.Equal(t, expected.ID, actual.ID)
	assert.Equal(t, expected.Info, actual.Info)
	assert.Equal(t, expected.Artifact, actual.Artifact)
	assert.True(t, time.Since(actual.CreatedAt) <= time.Second)
	assert.Zero(t, actual.CompletedAt)
	assert.True(t, actual.populated)
}

func TestTestResultsFind(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(testResultsCollection).Drop(ctx))
	}()

	tr1 := getTestResults()
	_, err := db.Collection(testResultsCollection).InsertOne(ctx, tr1)
	require.NoError(t, err)
	tr2 := getTestResults()
	_, err = db.Collection(testResultsCollection).InsertOne(ctx, tr2)
	require.NoError(t, err)

	t.Run("DNE", func(t *testing.T) {
		tr := TestResults{ID: "DNE"}
		tr.Setup(env)
		assert.Error(t, tr.Find(ctx))
		assert.False(t, tr.populated)
	})
	t.Run("NoEnv", func(t *testing.T) {
		tr := TestResults{ID: tr1.ID}
		assert.Error(t, tr.Find(ctx))
		assert.False(t, tr.populated)
	})
	t.Run("WithID", func(t *testing.T) {
		tr := TestResults{ID: tr1.ID}
		tr.Setup(env)
		require.NoError(t, tr.Find(ctx))
		assert.Equal(t, tr1.ID, tr.ID)
		assert.Equal(t, tr1.Info, tr.Info)
		assert.Equal(t, tr1.Artifact, tr.Artifact)
		assert.True(t, tr.populated)
	})
	t.Run("WithoutID", func(t *testing.T) {
		tr := TestResults{Info: tr2.Info}
		tr.Setup(env)
		require.NoError(t, tr.Find(ctx))
		assert.Equal(t, tr2.ID, tr.ID)
		assert.Equal(t, tr2.Info, tr.Info)
		assert.Equal(t, tr2.Artifact, tr.Artifact)
		assert.True(t, tr.populated)
	})
}

func TestTestResultsSaveNew(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(testResultsCollection).Drop(ctx))
	}()
	tr1 := getTestResults()
	tr2 := getTestResults()

	t.Run("NoEnv", func(t *testing.T) {
		tr := TestResults{
			ID:        tr1.ID,
			Info:      tr1.Info,
			CreatedAt: tr1.CreatedAt,
			Artifact:  tr1.Artifact,
			populated: true,
		}
		assert.Error(t, tr.SaveNew(ctx))
	})
	t.Run("Unpopulated", func(t *testing.T) {
		tr := TestResults{
			ID:        tr1.ID,
			Info:      tr1.Info,
			CreatedAt: tr1.CreatedAt,
			Artifact:  tr1.Artifact,
			populated: false,
		}
		tr.Setup(env)
		assert.Error(t, tr.SaveNew(ctx))
	})
	t.Run("WithID", func(t *testing.T) {
		saved := &TestResults{}
		require.Error(t, db.Collection(testResultsCollection).FindOne(ctx, bson.M{"_id": tr1.ID}).Decode(saved))

		tr := TestResults{
			ID:        tr1.ID,
			Info:      tr1.Info,
			CreatedAt: tr1.CreatedAt,
			Artifact:  tr1.Artifact,
			populated: true,
		}
		tr.Setup(env)
		require.NoError(t, tr.SaveNew(ctx))
		require.NoError(t, db.Collection(testResultsCollection).FindOne(ctx, bson.M{"_id": tr1.ID}).Decode(saved))
		assert.Equal(t, tr1.ID, saved.ID)
		assert.Equal(t, tr1.Info, saved.Info)
		assert.Equal(t, tr1.CreatedAt, saved.CreatedAt)
		assert.Equal(t, tr1.Artifact, saved.Artifact)
	})
	t.Run("WithoutID", func(t *testing.T) {
		saved := &TestResults{}
		require.Error(t, db.Collection(testResultsCollection).FindOne(ctx, bson.M{"_id": tr2.ID}).Decode(saved))

		tr := TestResults{
			Info:      tr2.Info,
			CreatedAt: tr2.CreatedAt,
			Artifact:  tr2.Artifact,
			populated: true,
		}
		tr.Setup(env)
		require.NoError(t, tr.SaveNew(ctx))
		require.NoError(t, db.Collection(testResultsCollection).FindOne(ctx, bson.M{"_id": tr2.ID}).Decode(saved))
		assert.Equal(t, tr2.ID, saved.ID)
		assert.Equal(t, tr2.Info, saved.Info)
		assert.Equal(t, tr2.CreatedAt, saved.CreatedAt)
		assert.Equal(t, tr2.Artifact, saved.Artifact)
	})
	t.Run("AlreadyExists", func(t *testing.T) {
		_, err := db.Collection(testResultsCollection).ReplaceOne(ctx, bson.M{"_id": tr2.ID}, tr2, options.Replace().SetUpsert(true))
		require.NoError(t, err)

		tr := TestResults{
			ID:        tr2.ID,
			populated: true,
		}
		tr.Setup(env)
		require.Error(t, tr.SaveNew(ctx))
	})
}

func TestTestResultsRemove(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(testResultsCollection).Drop(ctx))
	}()

	tr1 := getTestResults()
	_, err := db.Collection(testResultsCollection).InsertOne(ctx, tr1)
	require.NoError(t, err)
	tr2 := getTestResults()
	_, err = db.Collection(testResultsCollection).InsertOne(ctx, tr2)
	require.NoError(t, err)

	t.Run("NoEnv", func(t *testing.T) {
		tr := TestResults{
			ID: tr1.ID,
		}
		assert.Error(t, tr.Remove(ctx))
	})
	t.Run("DNE", func(t *testing.T) {
		tr := TestResults{ID: "DNE"}
		tr.Setup(env)
		require.NoError(t, tr.Remove(ctx))
	})
	t.Run("WithID", func(t *testing.T) {
		tr := TestResults{ID: tr1.ID}
		tr.Setup(env)
		require.NoError(t, tr.Remove(ctx))

		saved := &TestResults{}
		require.Error(t, db.Collection(testResultsCollection).FindOne(ctx, bson.M{"_id": tr1.ID}).Decode(saved))
	})
	t.Run("WithoutID", func(t *testing.T) {
		tr := TestResults{Info: tr2.Info}
		tr.Setup(env)
		require.NoError(t, tr.Remove(ctx))

		saved := &TestResults{}
		require.Error(t, db.Collection(testResultsCollection).FindOne(ctx, bson.M{"_id": tr2.ID}).Decode(saved))
	})
}

func TestTestResultsAppend(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tmpDir, err := ioutil.TempDir(".", "append-test")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
		assert.NoError(t, db.Collection(configurationCollection).Drop(ctx))
		assert.NoError(t, db.Collection(testResultsCollection).Drop(ctx))
	}()

	testBucket, err := pail.NewLocalBucket(pail.LocalOptions{Path: tmpDir})
	require.NoError(t, err)

	tr := getTestResults()
	_, err = db.Collection(testResultsCollection).InsertOne(ctx, tr)
	require.NoError(t, err)
	tr.populated = true
	results := []TestResult{}
	for i := 0; i < 10; i++ {
		result := getTestResult()
		result.TaskID = tr.Info.TaskID
		result.Execution = tr.Info.Execution
		results = append(results, result)
	}

	t.Run("NoEnv", func(t *testing.T) {
		tr.populated = true
		assert.Error(t, tr.Append(ctx, results))
	})
	t.Run("Unpopulated", func(t *testing.T) {
		tr.populated = false
		assert.Error(t, tr.Append(ctx, results))
	})
	tr.populated = true
	t.Run("NoConfig", func(t *testing.T) {
		tr.Setup(env)
		assert.Error(t, tr.Append(ctx, results))
	})
	conf := &CedarConfig{populated: true}
	conf.Setup(env)
	require.NoError(t, conf.Save())
	t.Run("ConfigWithoutBucket", func(t *testing.T) {
		tr.Setup(env)
		assert.Error(t, tr.Append(ctx, results))
	})
	conf.Bucket.TestResultsBucket = tmpDir
	conf.Bucket.PrestoBucket = tmpDir
	conf.Bucket.PrestoTestResultsPrefix = "presto-test-results"
	require.NoError(t, conf.Save())
	t.Run("AppendToBuckets", func(t *testing.T) {
		tr.Setup(env)
		require.NoError(t, tr.Append(ctx, results[0:5]))
		require.NoError(t, tr.Append(ctx, results[5:]))

		r, err := testBucket.Get(ctx, fmt.Sprintf("%s/%s", conf.Bucket.PrestoTestResultsPrefix, tr.PrestoPartitionKey()))
		require.NoError(t, err)
		data, err := ioutil.ReadAll(r)
		assert.NoError(t, r.Close())
		require.NoError(t, err)
		fr, err := goparquet.NewFileReader(bytes.NewReader(data))
		require.NoError(t, err)
		pr := floor.NewReader(fr)
		defer func() {
			assert.NoError(t, pr.Close())
		}()
		var parquetResults []ParquetTestResults
		for pr.Next() {
			row := ParquetTestResults{}
			require.NoError(t, pr.Scan(&row))
			parquetResults = append(parquetResults, row)
		}
		require.NoError(t, pr.Err())
		require.Len(t, parquetResults, 1)
		expectedParquet := ParquetTestResults{
			Version:     tr.Info.Version,
			Variant:     tr.Info.Variant,
			TaskName:    tr.Info.TaskName,
			TaskID:      tr.Info.TaskID,
			Execution:   int32(tr.Info.Execution),
			RequestType: tr.Info.RequestType,
			CreatedAt:   tr.CreatedAt.UTC(),
		}
		if tr.Info.DisplayTaskName != "" {
			expectedParquet.DisplayTaskName = utility.ToStringPtr(tr.Info.DisplayTaskName)
			expectedParquet.DisplayTaskID = utility.ToStringPtr(tr.Info.DisplayTaskID)
		}
		for _, result := range results {
			expectedParquet.Results = append(expectedParquet.Results, ParquetTestResult{
				TestName:       result.TestName,
				Trial:          int32(result.Trial),
				Status:         result.Status,
				LogInfo:        result.LogInfo,
				TaskCreateTime: result.TaskCreateTime.UTC(),
				TestStartTime:  result.TestStartTime.UTC(),
				TestEndTime:    result.TestEndTime.UTC(),
			})
			if result.DisplayTestName != "" {
				expectedParquet.Results[len(expectedParquet.Results)-1].DisplayTestName = utility.ToStringPtr(result.DisplayTestName)
				expectedParquet.Results[len(expectedParquet.Results)-1].GroupID = utility.ToStringPtr(result.GroupID)
				expectedParquet.Results[len(expectedParquet.Results)-1].LogTestName = utility.ToStringPtr(result.LogTestName)
				expectedParquet.Results[len(expectedParquet.Results)-1].LogURL = utility.ToStringPtr(result.LogURL)
				expectedParquet.Results[len(expectedParquet.Results)-1].RawLogURL = utility.ToStringPtr(result.RawLogURL)
				expectedParquet.Results[len(expectedParquet.Results)-1].LineNum = utility.ToInt32Ptr(int32(result.LineNum))
			}
		}
		assert.Equal(t, expectedParquet, parquetResults[0])

		// Check metadata.
		var saved TestResults
		require.NoError(t, db.Collection(testResultsCollection).FindOne(ctx, bson.M{"_id": tr.ID}).Decode(&saved))
		assert.Equal(t, len(results), saved.Stats.TotalCount)
		assert.Zero(t, saved.Stats.FailedCount)
		assert.Empty(t, saved.FailedTestsSample)

		failedResults := make([]TestResult, 2*FailedTestsSampleSize)
		for i := 0; i < 2*FailedTestsSampleSize; i++ {
			failedResults[i] = getTestResult()
			failedResults[i].TaskID = tr.Info.TaskID
			failedResults[i].Execution = tr.Info.Execution
			failedResults[i].Status = "Fail"
		}
		tr.Setup(env)
		require.NoError(t, tr.Append(ctx, failedResults[0:3]))
		require.NoError(t, tr.Append(ctx, failedResults[3:]))

		// Check Parquet data.
		r, err = testBucket.Get(ctx, fmt.Sprintf("%s/%s", conf.Bucket.PrestoTestResultsPrefix, tr.PrestoPartitionKey()))
		require.NoError(t, err)
		data, err = ioutil.ReadAll(r)
		assert.NoError(t, r.Close())
		require.NoError(t, err)
		fr, err = goparquet.NewFileReader(bytes.NewReader(data))
		require.NoError(t, err)
		pr = floor.NewReader(fr)
		defer func() {
			assert.NoError(t, pr.Close())
		}()
		parquetResults = []ParquetTestResults{}
		for pr.Next() {
			row := ParquetTestResults{}
			require.NoError(t, pr.Scan(&row))
			parquetResults = append(parquetResults, row)
		}
		require.NoError(t, pr.Err())
		require.Len(t, parquetResults, 1)
		for _, result := range failedResults {
			expectedParquet.Results = append(expectedParquet.Results, ParquetTestResult{
				TestName:       result.TestName,
				Trial:          int32(result.Trial),
				Status:         result.Status,
				LogInfo:        result.LogInfo,
				TaskCreateTime: result.TaskCreateTime.UTC(),
				TestStartTime:  result.TestStartTime.UTC(),
				TestEndTime:    result.TestEndTime.UTC(),
			})
			if result.DisplayTestName != "" {
				expectedParquet.Results[len(expectedParquet.Results)-1].DisplayTestName = utility.ToStringPtr(result.DisplayTestName)
				expectedParquet.Results[len(expectedParquet.Results)-1].GroupID = utility.ToStringPtr(result.GroupID)
				expectedParquet.Results[len(expectedParquet.Results)-1].LogTestName = utility.ToStringPtr(result.LogTestName)
				expectedParquet.Results[len(expectedParquet.Results)-1].LogURL = utility.ToStringPtr(result.LogURL)
				expectedParquet.Results[len(expectedParquet.Results)-1].RawLogURL = utility.ToStringPtr(result.RawLogURL)
				expectedParquet.Results[len(expectedParquet.Results)-1].LineNum = utility.ToInt32Ptr(int32(result.LineNum))
			}
		}
		assert.Equal(t, expectedParquet, parquetResults[0])

		// Check metadata.
		require.NoError(t, db.Collection(testResultsCollection).FindOne(ctx, bson.M{"_id": tr.ID}).Decode(&saved))
		assert.Equal(t, len(results)+len(failedResults), saved.Stats.TotalCount)
		assert.Equal(t, len(failedResults), saved.Stats.FailedCount)
		require.Len(t, saved.FailedTestsSample, FailedTestsSampleSize)
		for i, testName := range saved.FailedTestsSample {
			assert.Equal(t, failedResults[i].GetDisplayName(), testName)
		}
	})
}

func TestTestResultsDownload(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tmpDir := t.TempDir()
	defer func() {
		assert.NoError(t, db.Collection(configurationCollection).Drop(ctx))
		assert.NoError(t, db.Collection(testResultsCollection).Drop(ctx))
	}()

	tr := getTestResults()
	testBucket, err := pail.NewLocalBucket(pail.LocalOptions{Path: tmpDir})
	require.NoError(t, err)

	t.Run("NoEnv", func(t *testing.T) {
		tr.populated = true
		_, err = tr.Download(ctx)
		assert.Error(t, err)
	})
	t.Run("Unpopulated", func(t *testing.T) {
		tr.populated = false
		tr.Setup(env)
		_, err = tr.Download(ctx)
		assert.Error(t, err)
	})
	tr.populated = true
	t.Run("NoConfig", func(t *testing.T) {
		tr.Setup(env)
		_, err = tr.Download(ctx)
		assert.Error(t, err)
	})
	conf := &CedarConfig{populated: true}
	conf.Setup(env)
	require.NoError(t, conf.Save())
	t.Run("ConfigWithoutBucket", func(t *testing.T) {
		tr.Setup(env)
		_, err = tr.Download(ctx)
		assert.Error(t, err)
	})
	conf.Setup(env)
	require.NoError(t, conf.Find())
	conf.Bucket.TestResultsBucket = tmpDir
	conf.Bucket.PrestoBucket = tmpDir
	conf.Bucket.PrestoTestResultsPrefix = "presto-test-results"
	require.NoError(t, conf.Save())
	t.Run("DownloadFromBucketVersion1", func(t *testing.T) {
		expectedResults := make([]TestResult, 10)
		savedParquet := ParquetTestResults{
			Version:     tr.Info.Version,
			Variant:     tr.Info.Variant,
			TaskName:    tr.Info.TaskName,
			TaskID:      tr.Info.TaskID,
			Execution:   int32(tr.Info.Execution),
			RequestType: tr.Info.RequestType,
			CreatedAt:   tr.CreatedAt.UTC(),
			Results:     make([]ParquetTestResult, 10),
		}
		if tr.Info.DisplayTaskName != "" {
			savedParquet.DisplayTaskName = utility.ToStringPtr(tr.Info.DisplayTaskName)
			savedParquet.DisplayTaskID = utility.ToStringPtr(tr.Info.DisplayTaskID)
		}
		for i := 0; i < 10; i++ {
			result := getTestResult()
			expectedResults[i] = result
			expectedResults[i].TaskID = tr.Info.TaskID
			expectedResults[i].Execution = tr.Info.Execution
			savedParquet.Results[i] = ParquetTestResult{
				TestName:       result.TestName,
				Trial:          int32(result.Trial),
				GroupID:        utility.ToStringPtr(result.GroupID),
				Status:         result.Status,
				LogInfo:        result.LogInfo,
				TaskCreateTime: result.TaskCreateTime.UTC(),
				TestStartTime:  result.TestStartTime.UTC(),
				TestEndTime:    result.TestEndTime.UTC(),
			}
			if result.DisplayTestName != "" {
				savedParquet.Results[i].DisplayTestName = utility.ToStringPtr(result.DisplayTestName)
				savedParquet.Results[i].GroupID = utility.ToStringPtr(result.GroupID)
				savedParquet.Results[i].LogTestName = utility.ToStringPtr(result.LogTestName)
				savedParquet.Results[i].LogURL = utility.ToStringPtr(result.LogURL)
				savedParquet.Results[i].RawLogURL = utility.ToStringPtr(result.RawLogURL)
				savedParquet.Results[i].LineNum = utility.ToInt32Ptr(int32(result.LineNum))
			}
		}
		w, err := testBucket.Writer(ctx, fmt.Sprintf("%s/%s", conf.Bucket.PrestoTestResultsPrefix, tr.PrestoPartitionKey()))
		require.NoError(t, err)
		defer func() { assert.NoError(t, w.Close()) }()

		pw := floor.NewWriter(goparquet.NewFileWriter(w, goparquet.WithSchemaDefinition(parquetTestResultsSchemaDef)))
		require.NoError(t, pw.Write(savedParquet))
		require.NoError(t, pw.Close())

		tr.Setup(env)
		results, err := tr.Download(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedResults, results)
	})
	t.Run("DownloadFromBucketVersion0", func(t *testing.T) {
		tr0 := getTestResults()
		tr0.populated = true
		tr0.Artifact.Version = 0
		resultMap := map[string]TestResult{}
		for i := 0; i < 10; i++ {
			result := getTestResult()
			resultMap[result.TestName] = result
			var data []byte
			data, err = bson.Marshal(result)
			require.NoError(t, err)
			require.NoError(t, testBucket.Put(ctx, fmt.Sprintf("%s/%s", tr0.ID, result.TestName), bytes.NewReader(data)))
		}

		tr0.Setup(env)
		results, err := tr0.Download(ctx)
		require.NoError(t, err)

		require.Len(t, results, len(resultMap))
		for _, result := range results {
			expected, ok := resultMap[result.TestName]
			require.True(t, ok)
			assert.Equal(t, expected, result)
			delete(resultMap, result.TestName)
		}
	})
}

func TestTestResultsClose(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := env.Context()
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(testResultsCollection).Drop(ctx))
	}()

	tr1 := getTestResults()
	_, err := db.Collection(testResultsCollection).InsertOne(ctx, tr1)
	require.NoError(t, err)
	tr2 := getTestResults()
	_, err = db.Collection(testResultsCollection).InsertOne(ctx, tr2)
	require.NoError(t, err)

	t.Run("NoEnv", func(t *testing.T) {
		tr := &TestResults{ID: tr1.ID, populated: true}
		assert.Error(t, tr.Close(ctx))
	})
	t.Run("DNE", func(t *testing.T) {
		tr := &TestResults{ID: "DNE"}
		tr.Setup(env)
		assert.Error(t, tr.Close(ctx))
	})
	t.Run("WithID", func(t *testing.T) {
		tr := &TestResults{ID: tr1.ID, populated: true}
		tr.Setup(env)
		require.NoError(t, tr.Close(ctx))

		updated := &TestResults{}
		require.NoError(t, db.Collection(testResultsCollection).FindOne(ctx, bson.M{"_id": tr1.ID}).Decode(updated))
		assert.Equal(t, tr1.ID, updated.ID)
		assert.Equal(t, tr1.ID, updated.Info.ID())
		assert.Equal(t, tr1.CreatedAt, updated.CreatedAt)
		assert.True(t, time.Since(updated.CompletedAt) <= time.Second)
		assert.Equal(t, tr1.Info, updated.Info)
		assert.Equal(t, tr1.Artifact, updated.Artifact)
	})
	t.Run("WithoutID", func(t *testing.T) {
		tr := &TestResults{Info: tr2.Info, populated: true}
		tr.Setup(env)
		require.NoError(t, tr.Close(ctx))

		updated := &TestResults{}
		require.NoError(t, db.Collection(testResultsCollection).FindOne(ctx, bson.M{"_id": tr1.ID}).Decode(updated))
		assert.Equal(t, tr1.ID, updated.ID)
		assert.Equal(t, tr1.ID, updated.Info.ID())
		assert.Equal(t, tr1.CreatedAt, updated.CreatedAt)
		assert.True(t, time.Since(updated.CompletedAt) <= time.Second)
		assert.Equal(t, tr1.Info, updated.Info)
		assert.Equal(t, tr1.Artifact, updated.Artifact)
	})
}

func TestFindTestResults(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(testResultsCollection).Drop(ctx))
	}()

	tr1 := getTestResults()
	tr1.Info.Execution = 0
	_, err := db.Collection(testResultsCollection).InsertOne(ctx, tr1)
	require.NoError(t, err)

	tr2 := getTestResults()
	tr2.Info.TaskID = tr1.Info.TaskID
	tr2.Info.Execution = 1
	_, err = db.Collection(testResultsCollection).InsertOne(ctx, tr2)
	require.NoError(t, err)

	tr3 := getTestResults()
	tr3.Info.Execution = 0
	_, err = db.Collection(testResultsCollection).InsertOne(ctx, tr3)
	require.NoError(t, err)

	t.Run("NoTaskOptions", func(t *testing.T) {
		results, err := FindTestResults(ctx, env, nil)
		assert.Error(t, err)
		assert.Nil(t, results)
	})
	t.Run("NoEnv", func(t *testing.T) {
		opts := []TestResultsTaskOptions{
			{
				TaskID:    tr1.Info.TaskID,
				Execution: tr1.Info.Execution,
			},
		}
		results, err := FindTestResults(ctx, nil, opts)
		assert.Error(t, err)
		assert.Nil(t, results)
	})
	t.Run("TaskIDDNE", func(t *testing.T) {
		opts := []TestResultsTaskOptions{{TaskID: "DNE"}}
		results, err := FindTestResults(ctx, env, opts)
		assert.NoError(t, err)
		assert.Nil(t, results)
	})
	t.Run("SingleTaskID", func(t *testing.T) {
		opts := []TestResultsTaskOptions{
			{
				TaskID:    tr1.Info.TaskID,
				Execution: tr1.Info.Execution,
			},
		}
		results, err := FindTestResults(ctx, env, opts)
		require.NoError(t, err)
		require.NotEmpty(t, results)

		assert.Equal(t, tr1.ID, results[0].ID)
		assert.Equal(t, tr1.Info, results[0].Info)
		assert.Equal(t, tr1.Artifact, results[0].Artifact)
		assert.True(t, results[0].populated)
		assert.Equal(t, env, results[0].env)
	})
	t.Run("MultipleTaskIDs", func(t *testing.T) {
		opts := []TestResultsTaskOptions{
			{
				TaskID:    tr2.Info.TaskID,
				Execution: tr2.Info.Execution,
			},
			{
				TaskID:    tr3.Info.TaskID,
				Execution: tr3.Info.Execution,
			},
		}
		results, err := FindTestResults(ctx, env, opts)
		require.NoError(t, err)
		require.NotEmpty(t, results)

		count := 0
		for _, result := range results {
			if result.ID == tr2.ID {
				assert.Equal(t, tr2.ID, result.ID)
				assert.Equal(t, tr2.Info, result.Info)
				assert.Equal(t, tr2.Artifact, result.Artifact)
				assert.True(t, result.populated)
				assert.Equal(t, env, result.env)
				count++
			}
			if result.ID == tr3.ID {
				assert.Equal(t, tr3.ID, result.ID)
				assert.Equal(t, tr3.Info, result.Info)
				assert.Equal(t, tr3.Artifact, result.Artifact)
				assert.True(t, result.populated)
				assert.Equal(t, env, result.env)
				count++
			}
		}
		assert.Equal(t, 2, count)
	})
}

func TestFindAndDownloadTestResults(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tmpDir, err := ioutil.TempDir(".", "find-and-download-test")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
		assert.NoError(t, db.Collection(configurationCollection).Drop(ctx))
		assert.NoError(t, db.Collection(testResultsCollection).Drop(ctx))
	}()
	conf := &CedarConfig{
		Bucket: BucketConfig{
			TestResultsBucket:       tmpDir,
			PrestoBucket:            tmpDir,
			PrestoTestResultsPrefix: "presto-test-results",
		},
		populated: true,
	}
	conf.Setup(env)
	require.NoError(t, conf.Save())

	tr0 := getTestResults()
	tr0.Info.TaskID = "task0"
	tr0.Info.Execution = 0
	tr0.populated = true
	_, err = db.Collection(testResultsCollection).InsertOne(ctx, tr0)
	require.NoError(t, err)

	savedResults0 := make([]TestResult, 10)
	for i := 0; i < len(savedResults0); i++ {
		result := getTestResult()
		result.TaskID = tr0.Info.TaskID
		result.Execution = tr0.Info.Execution
		if i%2 != 0 {
			result.Status = "Fail"
		}
		savedResults0[i] = result
	}
	tr0.Setup(env)
	require.NoError(t, tr0.Append(ctx, savedResults0))

	tr1 := getTestResults()
	tr1.Info.TaskID = "task1"
	tr1.Info.Execution = 0
	tr1.populated = true
	_, err = db.Collection(testResultsCollection).InsertOne(ctx, tr1)
	require.NoError(t, err)

	savedResults1 := make([]TestResult, 10)
	for i := 0; i < len(savedResults1); i++ {
		result := getTestResult()
		result.TaskID = tr1.Info.TaskID
		result.Execution = tr1.Info.Execution
		savedResults1[i] = result
	}
	tr1.Setup(env)
	require.NoError(t, tr1.Append(ctx, savedResults1))

	tr2 := getTestResults()
	tr2.Info.TaskID = "task1"
	tr2.Info.Execution = 1
	tr2.populated = true
	_, err = db.Collection(testResultsCollection).InsertOne(ctx, tr2)
	require.NoError(t, err)

	savedResults2 := make([]TestResult, 10)
	for i := 0; i < len(savedResults2); i++ {
		result := getTestResult()
		result.TaskID = tr2.Info.TaskID
		result.Execution = tr2.Info.Execution
		savedResults2[i] = result
	}
	tr2.Setup(env)
	require.NoError(t, tr2.Append(ctx, savedResults2))

	t.Run("WithoutFilterAndSortOpts", func(t *testing.T) {
		taskOpts := []TestResultsTaskOptions{
			{
				TaskID:    tr1.Info.TaskID,
				Execution: tr1.Info.Execution,
			},
			{
				TaskID:    tr2.Info.TaskID,
				Execution: tr2.Info.Execution,
			},
			{
				TaskID:    tr0.Info.TaskID,
				Execution: tr0.Info.Execution,
			},
		}
		stats, results, err := FindAndDownloadTestResults(ctx, env, taskOpts, nil)
		require.NoError(t, err)

		assert.Equal(t, len(results), stats.TotalCount)
		assert.Equal(t, len(savedResults1)/2, stats.FailedCount)
		assert.Equal(t, len(results), utility.FromIntPtr(stats.FilteredCount))
		expectedResults := append(append(append([]TestResult{}, savedResults0...), savedResults1...), savedResults2...)
		assert.Equal(t, expectedResults, results)
	})
	t.Run("WithFilterAndSortOpts", func(t *testing.T) {
		taskOpts := []TestResultsTaskOptions{
			{
				TaskID:    tr0.Info.TaskID,
				Execution: tr0.Info.Execution,
			},
		}
		filterOpts := &TestResultsFilterAndSortOptions{Statuses: []string{"Pass"}}
		stats, results, err := FindAndDownloadTestResults(ctx, env, taskOpts, filterOpts)
		require.NoError(t, err)

		assert.Equal(t, len(savedResults0), stats.TotalCount)
		assert.Equal(t, len(savedResults0)/2, stats.FailedCount)
		assert.Equal(t, len(savedResults0)/2, utility.FromIntPtr(stats.FilteredCount))
		require.Len(t, results, len(savedResults0)/2)
		for i, result := range results {
			require.Equal(t, savedResults0[2*i], result)
		}
	})
}

func TestFindTestResultsStats(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(testResultsCollection).Drop(ctx))
	}()

	tr1 := getTestResults()
	tr1.Info.Execution = 0
	tr1.Stats.TotalCount = 10
	tr1.Stats.FailedCount = 5
	_, err := db.Collection(testResultsCollection).InsertOne(ctx, tr1)
	require.NoError(t, err)

	tr2 := getTestResults()
	tr2.Info.TaskID = tr1.Info.TaskID
	tr2.Info.Execution = 1
	tr2.Stats.TotalCount = 30
	tr2.Stats.FailedCount = 10
	_, err = db.Collection(testResultsCollection).InsertOne(ctx, tr2)
	require.NoError(t, err)

	tr3 := getTestResults()
	tr3.Info.Execution = 1
	tr3.Stats.TotalCount = 100
	tr3.Stats.FailedCount = 15
	_, err = db.Collection(testResultsCollection).InsertOne(ctx, tr3)
	require.NoError(t, err)

	tr4 := getTestResults()
	tr4.Info.Execution = 0
	tr4.Stats.TotalCount = 40
	tr4.Stats.FailedCount = 20
	_, err = db.Collection(testResultsCollection).InsertOne(ctx, tr4)
	require.NoError(t, err)

	for _, test := range []struct {
		name          string
		env           cedar.Environment
		opts          []TestResultsTaskOptions
		expectedStats TestResultsStats
		hasErr        bool
	}{
		{
			name:   "NoTaskOptions",
			env:    env,
			opts:   nil,
			hasErr: true,
		},
		{
			name:   "NilEnv",
			env:    nil,
			opts:   []TestResultsTaskOptions{{TaskID: tr1.Info.TaskID}},
			hasErr: true,
		},
		{
			name: "TaskIDDNE",
			env:  env,
			opts: []TestResultsTaskOptions{{TaskID: "DNE"}},
		},
		{
			name: "SingleTaskID",
			env:  env,
			opts: []TestResultsTaskOptions{
				{
					TaskID:    tr1.Info.TaskID,
					Execution: 0,
				},
			},
			expectedStats: tr1.Stats,
		},
		{
			name: "MultipleTaskIDs",
			env:  env,
			opts: []TestResultsTaskOptions{
				{
					TaskID:    tr1.Info.TaskID,
					Execution: 0,
				},
				{
					TaskID:    tr4.Info.TaskID,
					Execution: 0,
				},
			},
			expectedStats: TestResultsStats{
				TotalCount:  tr1.Stats.TotalCount + tr4.Stats.TotalCount,
				FailedCount: tr1.Stats.FailedCount + tr4.Stats.FailedCount,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			stats, err := FindTestResultsStats(ctx, test.env, test.opts)
			if test.hasErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expectedStats, stats)
			}
		})
	}
}

func TestFilterAndSortTestResults(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tmpDir, err := ioutil.TempDir(".", "filter-and-sort-test")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
		assert.NoError(t, db.Collection(configurationCollection).Drop(ctx))
		assert.NoError(t, db.Collection(testResultsCollection).Drop(ctx))
	}()
	conf := &CedarConfig{
		Bucket: BucketConfig{
			TestResultsBucket:       tmpDir,
			PrestoBucket:            tmpDir,
			PrestoTestResultsPrefix: "presto-test-results",
		},
		populated: true,
	}
	conf.Setup(env)
	require.NoError(t, conf.Save())

	getResults := func() []TestResult {
		return []TestResult{
			{
				TestName:      "A test",
				Status:        "Pass",
				TestStartTime: time.Date(1996, time.August, 31, 12, 5, 10, 1, time.UTC),
				TestEndTime:   time.Date(1996, time.August, 31, 12, 5, 12, 0, time.UTC),
			},
			{
				TestName:        "B test",
				DisplayTestName: "Display",
				Status:          "Fail",
				TestStartTime:   time.Date(1996, time.August, 31, 12, 5, 10, 3, time.UTC),
				TestEndTime:     time.Date(1996, time.August, 31, 12, 5, 16, 0, time.UTC),
			},
			{
				TestName:        "C test",
				DisplayTestName: "B",
				Status:          "Fail",
				TestStartTime:   time.Date(1996, time.August, 31, 12, 5, 10, 2, time.UTC),
				TestEndTime:     time.Date(1996, time.August, 31, 12, 5, 15, 0, time.UTC),
			},
			{
				TestName:      "D test",
				Status:        "Pass",
				TestStartTime: time.Date(1996, time.August, 31, 12, 5, 10, 4, time.UTC),
				TestEndTime:   time.Date(1996, time.August, 31, 12, 5, 11, 0, time.UTC),
				GroupID:       "llama",
			},
		}
	}
	results := getResults()

	base := getTestResults()
	base.populated = true
	_, err = db.Collection(testResultsCollection).InsertOne(ctx, base)
	require.NoError(t, err)
	baseResults := []TestResult{
		{
			TestName: "A test",
			Status:   "Pass",
		},
		{
			TestName:        "B test",
			DisplayTestName: "Display",
			Status:          "Fail",
		},
		{
			TestName:        "C test",
			DisplayTestName: "B",
			Status:          "Pass",
		},
		{
			TestName: "D test",
			Status:   "Fail",
		},
	}
	base.Setup(env)
	require.NoError(t, base.Append(ctx, baseResults))
	resultsWithBaseStatus := getResults()
	require.Len(t, resultsWithBaseStatus, len(baseResults))
	for i := range resultsWithBaseStatus {
		resultsWithBaseStatus[i].BaseStatus = baseResults[i].Status
	}

	for _, test := range []struct {
		name            string
		opts            *TestResultsFilterAndSortOptions
		expectedResults []TestResult
		expectedCount   int
		hasErr          bool
	}{
		{
			name: "InvalidSortByKey",
			opts: &TestResultsFilterAndSortOptions{
				Sort: []TestResultsSortBy{
					{Key: TestResultsSortByStatusKey},
					{Key: "invalid"},
				},
			},
			hasErr: true,
		},
		{
			name: "DuplicateSortByKey",
			opts: &TestResultsFilterAndSortOptions{
				Sort: []TestResultsSortBy{
					{Key: TestResultsSortByStatusKey},
					{Key: TestResultsSortByTestNameKey},
					{Key: TestResultsSortByStatusKey},
				},
			},
			hasErr: true,
		},
		{
			name: "SortByBaseStatusWithoutBaseTasksFindOptions",
			opts: &TestResultsFilterAndSortOptions{
				Sort: []TestResultsSortBy{{Key: TestResultsSortByBaseStatusKey}},
			},
			hasErr: true,
		},
		{
			name:   "NegativeLimit",
			opts:   &TestResultsFilterAndSortOptions{Limit: -1},
			hasErr: true,
		},
		{

			name: "NegativePage",
			opts: &TestResultsFilterAndSortOptions{
				Limit: 1,
				Page:  -1,
			},
			hasErr: true,
		},
		{
			name:   "PageWithoutLimit",
			opts:   &TestResultsFilterAndSortOptions{Page: 1},
			hasErr: true,
		},
		{
			name:   "InvalidTestNameRegex",
			opts:   &TestResultsFilterAndSortOptions{TestName: "*"},
			hasErr: true,
		},
		{
			name:            "EmptyOptions",
			expectedResults: results,
			expectedCount:   4,
		},
		{
			name: "TestNameRegexFilter",
			opts: &TestResultsFilterAndSortOptions{TestName: "A|B"},
			expectedResults: []TestResult{
				results[0],
				results[2],
			},
			expectedCount: 2,
		},
		{
			name: "TestNameExcludeDisplayNamesFilter",
			opts: &TestResultsFilterAndSortOptions{
				TestName:            "B",
				ExcludeDisplayNames: true,
			},
			expectedResults: results[1:2],
			expectedCount:   1,
		},
		{
			name:            "DisplayTestNameFilter",
			opts:            &TestResultsFilterAndSortOptions{TestName: "Di"},
			expectedResults: results[1:2],
			expectedCount:   1,
		},
		{
			name:            "StatusFilter",
			opts:            &TestResultsFilterAndSortOptions{Statuses: []string{"Fail"}},
			expectedResults: results[1:3],
			expectedCount:   2,
		},
		{
			name:            "GroupIDFilter",
			opts:            &TestResultsFilterAndSortOptions{GroupID: "llama"},
			expectedResults: results[3:4],
			expectedCount:   1,
		},
		{
			name: "SortByDurationASC",
			opts: &TestResultsFilterAndSortOptions{
				Sort: []TestResultsSortBy{{Key: TestResultsSortByDurationKey}},
			},
			expectedResults: []TestResult{
				results[3],
				results[0],
				results[2],
				results[1],
			},
			expectedCount: 4,
		},
		{
			name: "SortByDurationDSC",
			opts: &TestResultsFilterAndSortOptions{
				Sort: []TestResultsSortBy{
					{
						Key:      TestResultsSortByDurationKey,
						OrderDSC: true,
					},
				},
			},
			expectedResults: []TestResult{
				results[1],
				results[2],
				results[0],
				results[3],
			},
			expectedCount: 4,
		},
		{
			name: "SortByTestNameASC",
			opts: &TestResultsFilterAndSortOptions{
				Sort: []TestResultsSortBy{{Key: TestResultsSortByTestNameKey}},
			},
			expectedResults: []TestResult{
				results[0],
				results[2],
				results[3],
				results[1],
			},
			expectedCount: 4,
		},
		{
			name: "SortByTestNameDCS",
			opts: &TestResultsFilterAndSortOptions{
				Sort: []TestResultsSortBy{
					{
						Key:      TestResultsSortByTestNameKey,
						OrderDSC: true,
					},
				},
			},
			expectedResults: []TestResult{
				results[1],
				results[3],
				results[2],
				results[0],
			},
			expectedCount: 4,
		},
		{
			name: "SortByStatusASC",
			opts: &TestResultsFilterAndSortOptions{
				Sort: []TestResultsSortBy{{Key: TestResultsSortByStatusKey}},
			},
			expectedResults: []TestResult{
				results[1],
				results[2],
				results[0],
				results[3],
			},
			expectedCount: 4,
		},
		{
			name: "SortByStatusDSC",
			opts: &TestResultsFilterAndSortOptions{
				Sort: []TestResultsSortBy{
					{
						Key:      TestResultsSortByStatusKey,
						OrderDSC: true,
					},
				},
			},
			expectedResults: []TestResult{
				results[0],
				results[3],
				results[1],
				results[2],
			},
			expectedCount: 4,
		},
		{
			name: "SortByStartTimeASC",
			opts: &TestResultsFilterAndSortOptions{
				Sort: []TestResultsSortBy{{Key: TestResultsSortByStartKey}},
			},
			expectedResults: []TestResult{
				results[0],
				results[2],
				results[1],
				results[3],
			},
			expectedCount: 4,
		},
		{
			name: "SortByStartTimeDSC",
			opts: &TestResultsFilterAndSortOptions{
				Sort: []TestResultsSortBy{
					{
						Key:      TestResultsSortByStartKey,
						OrderDSC: true,
					},
				},
			},
			expectedResults: []TestResult{
				results[3],
				results[1],
				results[2],
				results[0],
			},
			expectedCount: 4,
		},
		{
			name: "SortByBaseStatusASC",
			opts: &TestResultsFilterAndSortOptions{
				Sort:      []TestResultsSortBy{{Key: TestResultsSortByBaseStatusKey}},
				BaseTasks: []TestResultsTaskOptions{{TaskID: base.Info.TaskID, Execution: base.Info.Execution}},
			},
			expectedResults: []TestResult{
				resultsWithBaseStatus[1],
				resultsWithBaseStatus[3],
				resultsWithBaseStatus[0],
				resultsWithBaseStatus[2],
			},
			expectedCount: 4,
		},
		{
			name: "SortByBaseStatusDSC",
			opts: &TestResultsFilterAndSortOptions{
				Sort: []TestResultsSortBy{
					{
						Key:      TestResultsSortByBaseStatusKey,
						OrderDSC: true,
					},
				},
				BaseTasks: []TestResultsTaskOptions{{TaskID: base.Info.TaskID, Execution: base.Info.Execution}},
			},
			expectedResults: []TestResult{
				resultsWithBaseStatus[0],
				resultsWithBaseStatus[2],
				resultsWithBaseStatus[1],
				resultsWithBaseStatus[3],
			},
			expectedCount: 4,
		},
		{
			name: "MultiSort",
			opts: &TestResultsFilterAndSortOptions{
				Sort: []TestResultsSortBy{
					{
						Key: TestResultsSortByStatusKey,
					},
					{
						Key:      TestResultsSortByTestNameKey,
						OrderDSC: true,
					},
				},
			},
			expectedResults: []TestResult{
				results[1],
				results[2],
				results[3],
				results[0],
			},
			expectedCount: 4,
		},
		{
			name: "BaseStatus",
			opts: &TestResultsFilterAndSortOptions{BaseTasks: []TestResultsTaskOptions{{TaskID: base.Info.TaskID, Execution: base.Info.Execution}}},
			expectedResults: []TestResult{
				resultsWithBaseStatus[0],
				resultsWithBaseStatus[1],
				resultsWithBaseStatus[2],
				resultsWithBaseStatus[3],
			},
			expectedCount: 4,
		},
		{
			name:            "Limit",
			opts:            &TestResultsFilterAndSortOptions{Limit: 3},
			expectedResults: results[0:3],
			expectedCount:   4,
		},
		{
			name: "LimitAndPage",
			opts: &TestResultsFilterAndSortOptions{
				Limit: 3,
				Page:  1,
			},
			expectedResults: results[3:],
			expectedCount:   4,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			actualResults, count, err := filterAndSortTestResults(context.TODO(), env, getResults(), test.opts)
			if test.hasErr {
				assert.Nil(t, actualResults)
				assert.Zero(t, count)
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expectedResults, actualResults)
				assert.Equal(t, test.expectedCount, count)
			}
		})
	}
}

func TestFindFailedTestResultsSamples(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(testResultsCollection).Drop(ctx))
	}()

	t0_0 := TestResults{
		Info:              TestResultsInfo{TaskID: "t0"},
		FailedTestsSample: []string{"test1", "test2"},
	}
	t0_2 := TestResults{
		Info:              TestResultsInfo{TaskID: "t0", Execution: 2},
		FailedTestsSample: []string{"test1", "test3", "test44"},
	}
	t1_0 := TestResults{
		Info:              TestResultsInfo{TaskID: "t1"},
		FailedTestsSample: []string{"test2"},
	}
	t1_1 := TestResults{
		Info: TestResultsInfo{TaskID: "t1", Execution: 1},
	}
	_, err := db.Collection(testResultsCollection).InsertMany(ctx, []interface{}{t0_0, t0_2, t1_0, t1_1})
	require.NoError(t, err)

	for _, test := range []struct {
		name     string
		tasks    []TestResultsTaskOptions
		filters  []string
		expected []TestResultsSample
		hasErr   bool
	}{
		{
			name:   "NilTaskOptions",
			hasErr: true,
		},
		{
			name:   "NoTaskOptions",
			tasks:  []TestResultsTaskOptions{},
			hasErr: true,
		},
		{
			name:    "InvalidRegexHasError",
			tasks:   []TestResultsTaskOptions{{TaskID: "t1"}},
			filters: []string{`[`},
			hasErr:  true,
		},
		{
			name:     "NoResults",
			tasks:    []TestResultsTaskOptions{{TaskID: "DNE"}},
			expected: []TestResultsSample{},
		},
		{
			name: "WithoutFilters",
			tasks: []TestResultsTaskOptions{
				{TaskID: t0_0.Info.TaskID, Execution: t0_0.Info.Execution},
				{TaskID: t0_2.Info.TaskID, Execution: t0_2.Info.Execution},
				{TaskID: t1_0.Info.TaskID, Execution: t1_0.Info.Execution},
				{TaskID: t1_1.Info.TaskID, Execution: t1_1.Info.Execution},
			},
			expected: []TestResultsSample{
				{
					TaskID:                  t0_0.Info.TaskID,
					Execution:               t0_0.Info.Execution,
					MatchingFailedTestNames: t0_0.FailedTestsSample,
					TotalFailedTestNames:    len(t0_0.FailedTestsSample),
				},
				{
					TaskID:                  t0_2.Info.TaskID,
					Execution:               t0_2.Info.Execution,
					MatchingFailedTestNames: t0_2.FailedTestsSample,
					TotalFailedTestNames:    len(t0_2.FailedTestsSample),
				},
				{
					TaskID:                  t1_0.Info.TaskID,
					Execution:               t1_0.Info.Execution,
					MatchingFailedTestNames: t1_0.FailedTestsSample,
					TotalFailedTestNames:    len(t1_0.FailedTestsSample),
				},
				{
					TaskID:    t1_1.Info.TaskID,
					Execution: t1_1.Info.Execution,
				},
			},
		},
		{
			name: "WithFilters",
			tasks: []TestResultsTaskOptions{
				{TaskID: t0_0.Info.TaskID, Execution: t0_0.Info.Execution},
				{TaskID: t0_2.Info.TaskID, Execution: t0_2.Info.Execution},
				{TaskID: t1_0.Info.TaskID, Execution: t1_0.Info.Execution},
				{TaskID: t1_1.Info.TaskID, Execution: t1_1.Info.Execution},
			},
			filters: []string{"test1", "test3", "test4", "test44"},
			expected: []TestResultsSample{
				{
					TaskID:                  t0_0.Info.TaskID,
					Execution:               t0_0.Info.Execution,
					MatchingFailedTestNames: []string{"test1"},
					TotalFailedTestNames:    len(t0_0.FailedTestsSample),
				},
				{
					TaskID:                  t0_2.Info.TaskID,
					Execution:               t0_2.Info.Execution,
					MatchingFailedTestNames: []string{"test1", "test3", "test44"},
					TotalFailedTestNames:    len(t0_2.FailedTestsSample),
				},
				{
					TaskID:               t1_0.Info.TaskID,
					Execution:            t1_0.Info.Execution,
					TotalFailedTestNames: len(t1_0.FailedTestsSample),
				},
				{
					TaskID:    t1_1.Info.TaskID,
					Execution: t1_1.Info.Execution,
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			samples, err := FindFailedTestResultsSamples(ctx, env, test.tasks, test.filters)
			if test.hasErr {
				assert.Error(t, err)
				assert.Nil(t, samples)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expected, samples)
			}
		})
	}
}

func getTestResults() *TestResults {
	info := TestResultsInfo{
		Project:     utility.RandomString(),
		Version:     utility.RandomString(),
		Variant:     utility.RandomString(),
		TaskName:    utility.RandomString(),
		TaskID:      utility.RandomString(),
		Execution:   rand.Intn(5),
		RequestType: utility.RandomString(),
	}
	// Optional fields, we should test that we handle them properly when
	// they are populated and when they do not.
	if sometimes.Half() {
		info.DisplayTaskName = utility.RandomString()
		info.DisplayTaskID = utility.RandomString()
	}

	return &TestResults{
		ID:          info.ID(),
		Info:        info,
		CreatedAt:   time.Now().Add(-time.Hour).UTC().Round(time.Millisecond),
		CompletedAt: time.Now().UTC().Round(time.Millisecond),
		Artifact: TestResultsArtifactInfo{
			Type:    PailLocal,
			Prefix:  info.ID(),
			Version: 1,
		},
	}
}

func getTestResult() TestResult {
	result := TestResult{
		TestName:       utility.RandomString(),
		Trial:          rand.Intn(10),
		Status:         "Pass",
		TaskCreateTime: time.Now().Add(-time.Hour).UTC().Round(time.Millisecond),
		TestStartTime:  time.Now().Add(-30 * time.Hour).UTC().Round(time.Millisecond),
		TestEndTime:    time.Now().UTC().Round(time.Millisecond),
	}
	// Optional fields, we should test that we handle them properly when
	// they are populated and when they do not.
	if sometimes.Half() {
		result.DisplayTestName = utility.RandomString()
		result.GroupID = utility.RandomString()
		result.LogTestName = utility.RandomString()
		result.LogURL = utility.RandomString()
		result.RawLogURL = utility.RandomString()
		result.LineNum = rand.Intn(1000)
		result.LogInfo = &TestLogInfo{
			LogName:       utility.RandomString(),
			LogsToMerge:   []string{utility.RandomString(), utility.RandomString()},
			LineNum:       rand.Int31n(1000),
			RenderingType: utility.ToStringPtr(utility.RandomString()),
			Version:       rand.Int31n(5),
		}
	}

	return result
}
