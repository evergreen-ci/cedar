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
		results = append(results, getTestResult())
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
	conf.Setup(env)
	require.NoError(t, conf.Find())
	conf.Bucket.TestResultsBucket = tmpDir
	require.NoError(t, conf.Save())
	t.Run("AppendToBucket", func(t *testing.T) {
		tr.Setup(env)
		require.NoError(t, tr.Append(ctx, results[0:5]))
		require.NoError(t, tr.Append(ctx, results[5:]))

		var savedResults testResultsDoc
		r, err := testBucket.Get(ctx, fmt.Sprintf("%s/%s", tr.ID, testResultsCollection))
		require.NoError(t, err)
		data, err := ioutil.ReadAll(r)
		assert.NoError(t, r.Close())
		require.NoError(t, err)
		require.NoError(t, bson.Unmarshal(data, &savedResults))
		assert.Equal(t, results, savedResults.Results)
		var saved TestResults
		require.NoError(t, db.Collection(testResultsCollection).FindOne(ctx, bson.M{"_id": tr.ID}).Decode(&saved))
		assert.Equal(t, len(results), saved.Stats.TotalCount)
		assert.Zero(t, saved.Stats.FailedCount)
		assert.Empty(t, saved.FailedTestsSample)

		failedResults := make([]TestResult, 2*FailedTestsSampleSize)
		for i := 0; i < 2*FailedTestsSampleSize; i++ {
			failedResults[i] = getTestResult()
			failedResults[i].Status = "fail"
		}
		tr.Setup(env)
		require.NoError(t, tr.Append(ctx, failedResults[0:3]))
		require.NoError(t, tr.Append(ctx, failedResults[3:]))

		r, err = testBucket.Get(ctx, fmt.Sprintf("%s/%s", tr.ID, testResultsCollection))
		require.NoError(t, err)
		data, err = ioutil.ReadAll(r)
		assert.NoError(t, r.Close())
		require.NoError(t, err)
		require.NoError(t, bson.Unmarshal(data, &savedResults))
		assert.Equal(t, append(results, failedResults...), savedResults.Results)
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
	tmpDir, err := ioutil.TempDir(".", "download-test")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
		assert.NoError(t, db.Collection(configurationCollection).Drop(ctx))
		assert.NoError(t, db.Collection(testResultsCollection).Drop(ctx))
	}()

	tr := getTestResults()
	testBucket, err := pail.NewLocalBucket(pail.LocalOptions{Path: tmpDir, Prefix: tr.ID})
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
	require.NoError(t, conf.Save())
	t.Run("DownloadFromBucketVersion1", func(t *testing.T) {
		savedResults := testResultsDoc{}
		savedResults.Results = make([]TestResult, 10)
		for i := 0; i < 10; i++ {
			savedResults.Results[i] = getTestResult()
		}
		data, err := bson.Marshal(&savedResults)
		require.NoError(t, err)
		require.NoError(t, testBucket.Put(ctx, testResultsCollection, bytes.NewReader(data)))

		tr.Setup(env)
		results, err := tr.Download(ctx)
		require.NoError(t, err)
		assert.Equal(t, savedResults.Results, results)
	})
	t.Run("DownloadFromBucketVersion0", func(t *testing.T) {
		tr0 := getTestResults()
		tr0.populated = true
		tr0.Artifact.Version = 0
		testBucket0, err := pail.NewLocalBucket(pail.LocalOptions{Path: tmpDir, Prefix: tr0.ID})
		require.NoError(t, err)
		resultMap := map[string]TestResult{}
		for i := 0; i < 10; i++ {
			result := getTestResult()
			resultMap[result.TestName] = result
			var data []byte
			data, err = bson.Marshal(result)
			require.NoError(t, err)
			require.NoError(t, testBucket0.Put(ctx, result.TestName, bytes.NewReader(data)))
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
	tr1.Info.DisplayTaskID = "display"
	tr1.Info.Execution = 0
	_, err := db.Collection(testResultsCollection).InsertOne(ctx, tr1)
	require.NoError(t, err)

	tr2 := getTestResults()
	tr2.Info.DisplayTaskID = "display"
	tr2.Info.TaskID = tr1.Info.TaskID
	tr2.Info.Execution = 1
	_, err = db.Collection(testResultsCollection).InsertOne(ctx, tr2)
	require.NoError(t, err)

	tr3 := getTestResults()
	tr3.Info.DisplayTaskID = "display"
	tr3.Info.Execution = 0
	_, err = db.Collection(testResultsCollection).InsertOne(ctx, tr3)
	require.NoError(t, err)

	t.Run("NoTaskID", func(t *testing.T) {
		opts := TestResultsFindOptions{}
		results, err := FindTestResults(ctx, env, opts)
		assert.Error(t, err)
		assert.Nil(t, results)
	})
	t.Run("NegativeExecution", func(t *testing.T) {
		opts := TestResultsFindOptions{
			TaskID:    tr1.Info.TaskID,
			Execution: utility.ToIntPtr(-1),
		}
		results, err := FindTestResults(ctx, env, opts)
		assert.Error(t, err)
		assert.Nil(t, results)
	})
	t.Run("TaskIDDNE", func(t *testing.T) {
		opts := TestResultsFindOptions{
			TaskID: "DNE",
		}
		results, err := FindTestResults(ctx, env, opts)
		assert.Error(t, err)
		assert.Nil(t, results)
	})
	t.Run("DisplayTaskIDDNE", func(t *testing.T) {
		opts := TestResultsFindOptions{
			TaskID:      "DNE",
			DisplayTask: true,
		}
		results, err := FindTestResults(ctx, env, opts)
		assert.Error(t, err)
		assert.Nil(t, results)
	})
	t.Run("NoEnv", func(t *testing.T) {
		opts := TestResultsFindOptions{
			TaskID:    tr1.Info.TaskID,
			Execution: utility.ToIntPtr(tr1.Info.Execution),
		}
		results, err := FindTestResults(ctx, nil, opts)
		assert.Error(t, err)
		assert.Nil(t, results)
	})
	t.Run("WithTaskIDAndExecution", func(t *testing.T) {
		opts := TestResultsFindOptions{
			TaskID:    tr1.Info.TaskID,
			Execution: utility.ToIntPtr(tr1.Info.Execution),
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
	t.Run("WithTaskIDWithoutExecution", func(t *testing.T) {
		opts := TestResultsFindOptions{
			TaskID: tr2.Info.TaskID,
		}
		results, err := FindTestResults(ctx, env, opts)
		require.NoError(t, err)
		require.NotEmpty(t, results)
		assert.Equal(t, tr2.ID, results[0].ID)
		assert.Equal(t, tr2.Info, results[0].Info)
		assert.Equal(t, tr2.Artifact, results[0].Artifact)
		assert.True(t, results[0].populated)
		assert.Equal(t, env, results[0].env)
	})
	t.Run("WithDisplayTaskIDAndExecution", func(t *testing.T) {
		opts := TestResultsFindOptions{
			TaskID:      "display",
			Execution:   utility.ToIntPtr(0),
			DisplayTask: true,
		}
		results, err := FindTestResults(ctx, env, opts)
		require.NoError(t, err)
		count := 0
		for _, result := range results {
			if result.ID == tr1.ID {
				assert.Equal(t, tr1.ID, result.ID)
				assert.Equal(t, tr1.Info, result.Info)
				assert.Equal(t, tr1.Artifact, result.Artifact)
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
	t.Run("WithDisplayTaskIDWithoutExecution", func(t *testing.T) {
		opts := TestResultsFindOptions{
			TaskID:      "display",
			DisplayTask: true,
		}
		results, err := FindTestResults(ctx, env, opts)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, tr2.ID, results[0].ID)
		assert.Equal(t, tr2.Info, results[0].Info)
		assert.Equal(t, tr2.Artifact, results[0].Artifact)
		assert.True(t, results[0].populated)
		assert.Equal(t, env, results[0].env)
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
		Bucket:    BucketConfig{TestResultsBucket: tmpDir},
		populated: true,
	}
	conf.Setup(env)
	require.NoError(t, conf.Save())

	tr1 := getTestResults()
	tr1.Info.DisplayTaskID = "display"
	tr1.Info.Execution = 0
	_, err = db.Collection(testResultsCollection).InsertOne(ctx, tr1)
	require.NoError(t, err)
	testBucket1, err := pail.NewLocalBucket(pail.LocalOptions{Path: tmpDir, Prefix: tr1.ID})
	require.NoError(t, err)

	savedResults1 := testResultsDoc{}
	for i := 0; i < 10; i++ {
		savedResults1.Results = append(savedResults1.Results, getTestResult())
	}
	data, err := bson.Marshal(&savedResults1)
	require.NoError(t, err)
	require.NoError(t, testBucket1.Put(ctx, testResultsCollection, bytes.NewReader(data)))

	tr2 := getTestResults()
	tr2.Info.DisplayTaskID = "display"
	tr2.Info.Execution = 0
	_, err = db.Collection(testResultsCollection).InsertOne(ctx, tr2)
	require.NoError(t, err)
	testBucket2, err := pail.NewLocalBucket(pail.LocalOptions{Path: tmpDir, Prefix: tr2.ID})
	require.NoError(t, err)

	savedResults2 := testResultsDoc{}
	for i := 0; i < 10; i++ {
		savedResults2.Results = append(savedResults2.Results, getTestResult())
	}
	data, err = bson.Marshal(&savedResults2)
	require.NoError(t, err)
	require.NoError(t, testBucket2.Put(ctx, testResultsCollection, bytes.NewReader(data)))

	opts := TestResultsFindOptions{
		TaskID:      "display",
		DisplayTask: true,
	}
	results, err := FindAndDownloadTestResults(ctx, env, opts)
	require.NoError(t, err)

	require.Len(t, results, len(savedResults1.Results)+len(savedResults2.Results))
	for _, result := range append(savedResults1.Results, savedResults2.Results...) {
		assert.Contains(t, results, result)
	}
}

func TestGetTestResultsStats(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(testResultsCollection).Drop(ctx))
	}()

	tr1 := getTestResults()
	tr1.Info.DisplayTaskID = "display"
	tr1.Info.Execution = 0
	tr1.Stats.TotalCount = 10
	tr1.Stats.FailedCount = 5
	_, err := db.Collection(testResultsCollection).InsertOne(ctx, tr1)
	require.NoError(t, err)

	tr2 := getTestResults()
	tr2.Info.DisplayTaskID = "display"
	tr2.Info.TaskID = tr1.Info.TaskID
	tr2.Info.Execution = 1
	tr2.Stats.TotalCount = 30
	tr2.Stats.FailedCount = 10
	_, err = db.Collection(testResultsCollection).InsertOne(ctx, tr2)
	require.NoError(t, err)

	tr3 := getTestResults()
	tr3.Info.DisplayTaskID = "display"
	tr3.Info.Execution = 1
	tr3.Stats.TotalCount = 100
	tr3.Stats.FailedCount = 15
	_, err = db.Collection(testResultsCollection).InsertOne(ctx, tr3)
	require.NoError(t, err)

	tr4 := getTestResults()
	tr4.Info.DisplayTaskID = "display"
	tr4.Info.Execution = 0
	tr4.Stats.TotalCount = 40
	tr4.Stats.FailedCount = 20
	_, err = db.Collection(testResultsCollection).InsertOne(ctx, tr4)
	require.NoError(t, err)

	for _, test := range []struct {
		name          string
		env           cedar.Environment
		opts          TestResultsFindOptions
		expectedStats TestResultsStats
		hasErr        bool
	}{
		{
			name: "FailsWithNoTaskID",
			env:  env,
			// Set DisplayTask to true to check that the function
			// does its own validation, otherwise, if DisplayTask
			// is false, the function just calls FindTestResults
			// and the options validation is done there.
			opts:   TestResultsFindOptions{DisplayTask: true},
			hasErr: true,
		},
		{
			name: "FailsWithNegativeExecution",
			env:  env,
			opts: TestResultsFindOptions{
				TaskID:      tr1.Info.DisplayTaskID,
				DisplayTask: true,
				Execution:   utility.ToIntPtr(-1),
			},
			hasErr: true,
		},
		{
			name: "FailsWithNilEnv",
			env:  nil,
			opts: TestResultsFindOptions{
				TaskID:      tr1.Info.DisplayTaskID,
				DisplayTask: true,
			},
			hasErr: true,
		},
		{
			name:   "FailsWhenTaskIDDNE",
			env:    env,
			opts:   TestResultsFindOptions{TaskID: "DNE"},
			hasErr: true,
		},
		{
			name: "FailsWhenDisplayTaskIDDNE",
			env:  env,
			opts: TestResultsFindOptions{
				TaskID:      "DNE",
				DisplayTask: true,
			},
			hasErr: true,
		},
		{
			name: "SucceedsWithTaskIDAndExecution",
			env:  env,
			opts: TestResultsFindOptions{
				TaskID:    tr1.Info.TaskID,
				Execution: utility.ToIntPtr(0),
			},
			expectedStats: tr1.Stats,
		},
		{
			name:          "SucceedsWithTaskIDAndNoExecution",
			env:           env,
			opts:          TestResultsFindOptions{TaskID: tr1.Info.TaskID},
			expectedStats: tr2.Stats,
		},
		{
			name: "SucceedsWithDisplayTaskIDAndExecution",
			env:  env,
			opts: TestResultsFindOptions{
				TaskID:      "display",
				Execution:   utility.ToIntPtr(0),
				DisplayTask: true,
			},
			expectedStats: TestResultsStats{
				TotalCount:  tr1.Stats.TotalCount + tr4.Stats.TotalCount,
				FailedCount: tr1.Stats.FailedCount + tr4.Stats.FailedCount,
			},
		},
		{
			name: "SucceedsWithDisplayTaskIDAndNoExecution",
			env:  env,
			opts: TestResultsFindOptions{
				TaskID:      "display",
				DisplayTask: true,
			},
			expectedStats: TestResultsStats{
				TotalCount:  tr2.Stats.TotalCount + tr3.Stats.TotalCount,
				FailedCount: tr2.Stats.FailedCount + tr3.Stats.FailedCount,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			stats, err := GetTestResultsStats(ctx, test.env, test.opts)
			if test.hasErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expectedStats, stats)
			}
		})
	}
}

func getTestResults() *TestResults {
	info := TestResultsInfo{
		Project:                utility.RandomString(),
		Version:                utility.RandomString(),
		Variant:                utility.RandomString(),
		TaskName:               utility.RandomString(),
		TaskID:                 utility.RandomString(),
		Execution:              rand.Intn(5),
		RequestType:            utility.RandomString(),
		HistoricalDataDisabled: true,
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
	return TestResult{
		TestName:        utility.RandomString(),
		DisplayTestName: utility.RandomString(),
		GroupID:         utility.RandomString(),
		Trial:           rand.Intn(10),
		Status:          "Pass",
		LogTestName:     utility.RandomString(),
		LogURL:          utility.RandomString(),
		RawLogURL:       utility.RandomString(),
		LineNum:         rand.Intn(1000),
		TaskCreateTime:  time.Now().Add(-time.Hour).UTC().Round(time.Millisecond),
		TestStartTime:   time.Now().Add(-30 * time.Hour).UTC().Round(time.Millisecond),
		TestEndTime:     time.Now().UTC().Round(time.Millisecond),
	}
}
