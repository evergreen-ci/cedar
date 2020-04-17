package model

import (
	"bytes"
	"context"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/pail"
	"github.com/evergreen-ci/utility"
	"github.com/jpillora/backoff"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

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
	})
	t.Run("NoEnv", func(t *testing.T) {
		tr := TestResults{ID: tr1.ID}
		assert.Error(t, tr.Find(ctx))
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
		_, err := db.Collection(buildloggerCollection).ReplaceOne(ctx, bson.M{"_id": tr2.ID}, tr2, options.Replace().SetUpsert(true))
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
	}()

	testBucket, err := pail.NewLocalBucket(pail.LocalOptions{Path: tmpDir})
	require.NoError(t, err)

	tr := getTestResults()
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

		b := &backoff.Backoff{
			Min:    100 * time.Millisecond,
			Max:    5 * time.Second,
			Factor: 2,
		}
		var savedResults map[string]TestResult
		for i := 0; i < 10; i++ {
			savedResults = map[string]TestResult{}
			iter, err := testBucket.List(ctx, tr.ID)
			require.NoError(t, err)
			for iter.Next(ctx) {
				require.NoError(t, err)
				r, err := iter.Item().Get(ctx)
				require.NoError(t, err)
				defer func() {
					assert.NoError(t, r.Close())
				}()
				data, err := ioutil.ReadAll(r)
				require.NoError(t, err)
				result := TestResult{}
				require.NoError(t, bson.Unmarshal(data, &result))
				savedResults[result.TestName] = result
			}

			if len(savedResults) == 10 {
				break
			}
			time.Sleep(b.Duration())
		}
		require.Len(t, savedResults, 10)
		for _, result := range results {
			savedResult, ok := savedResults[result.TestName]
			require.True(t, ok)
			assert.Equal(t, result, savedResult)
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
	}()

	tr := getTestResults()
	testBucket, err := pail.NewLocalBucket(pail.LocalOptions{Path: tmpDir, Prefix: tr.ID})
	require.NoError(t, err)

	results := []TestResult{}
	resultsMap := map[string]TestResult{}
	for i := 0; i < 10; i++ {
		result := getTestResult()
		results = append(results, result)
		resultsMap[result.TestName] = result
		data, err := bson.Marshal(result)
		require.NoError(t, err)
		require.NoError(t, testBucket.Put(ctx, result.TestName, bytes.NewReader(data)))
	}

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
	t.Run("DownloadFromBucket", func(t *testing.T) {
		tr.Setup(env)

		b := &backoff.Backoff{
			Min:    100 * time.Millisecond,
			Max:    5 * time.Second,
			Factor: 2,
		}

		var results []TestResult
		for i := 0; i < 10; i++ {
			results, err = tr.Download(ctx)
			require.NoError(t, err)
			if len(results) == 10 {
				break
			}

			time.Sleep(b.Duration())
		}

		require.Len(t, results, 10)
		for _, result := range results {
			expected, ok := resultsMap[result.TestName]
			require.True(t, ok)
			assert.Equal(t, expected, result)
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

func getTestResults() *TestResults {
	info := TestResultsInfo{
		Project:   utility.RandomString(),
		Version:   utility.RandomString(),
		Variant:   utility.RandomString(),
		TaskName:  utility.RandomString(),
		TaskID:    utility.RandomString(),
		Execution: seedRand.Intn(5),
		Requester: utility.RandomString(),
	}
	return &TestResults{
		ID:          info.ID(),
		Info:        info,
		CreatedAt:   time.Now().Add(-time.Hour).UTC().Round(time.Millisecond),
		CompletedAt: time.Now().UTC().Round(time.Millisecond),
		Artifact: TestResultsArtifactInfo{
			Type:   PailLocal,
			Prefix: info.ID(),
		},
	}
}

func getTestResult() TestResult {
	return TestResult{
		TestName:       utility.RandomString(),
		Trial:          seedRand.Intn(10),
		Status:         "Pass",
		LineNum:        seedRand.Intn(1000),
		TaskCreateTime: time.Now().Add(-time.Hour).UTC().Round(time.Millisecond),
		TestStartTime:  time.Now().Add(-30 * time.Hour).UTC().Round(time.Millisecond),
		TestEndTime:    time.Now().UTC().Round(time.Millisecond),
	}
}

var seedRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))
