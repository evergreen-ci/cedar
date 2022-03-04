package units

import (
	"context"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/anser/bsonutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestBSONToParquetBackfillJob(t *testing.T) {
	env := cedar.GetEnvironment()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tmpDir, err := ioutil.TempDir(".", "bson-to-parquet-test")
	require.NoError(t, err)

	defer func() {
		assert.NoError(t, tearDownEnv(env))
		assert.NoError(t, os.RemoveAll(tmpDir))
	}()

	config := model.NewCedarConfig(env)
	config.Bucket.TestResultsBucket = tmpDir
	config.Bucket.PrestoBucket = tmpDir
	config.Bucket.PrestoTestResultsPrefix = "presto-test-results"
	require.NoError(t, config.Save())

	controller := model.BatchJobController{
		ID:         bsonToParquetBackfillJobName,
		Collection: "backfill_test_results",
		Iterations: 10,
		BatchSize:  10,
	}
	numDocs := controller.Iterations * controller.BatchSize
	updateController(t, ctx, env, controller)
	populateTestResults(t, ctx, env, controller.Collection, numDocs)

	j1 := NewBSONToParquetBackfillJob("all-nil")
	j1.Run(ctx)
	require.NoError(t, j1.Error())

	cur, err := env.GetDB().Collection(controller.Collection).Find(
		ctx,
		bson.M{bsonutil.GetDottedKeyName(model.TestResultsMigrationKey, model.MigrationStatsMigratorIDKey): j1.ID()},
	)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, cur.Close(ctx))
	}()
	var results []model.TestResults
	require.NoError(t, cur.All(ctx, &results))

	assert.Len(t, results, controller.BatchSize*controller.Iterations)
	for _, result := range results {
		assert.NotNil(t, result.Migration.StartedAt)
		assert.NotNil(t, result.Migration.CompletedAt)
		assert.Equal(t, controller.Version, result.Migration.Version)
	}

	// Update batch controller version.
	controller.Version = 1
	updateController(t, ctx, env, controller)
	j2 := NewBSONToParquetBackfillJob("new-version")
	j2.Run(ctx)
	require.NoError(t, j2.Error())

	cur, err = env.GetDB().Collection(controller.Collection).Find(
		ctx,
		bson.M{bsonutil.GetDottedKeyName(model.TestResultsMigrationKey, model.MigrationStatsMigratorIDKey): j2.ID()},
	)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, cur.Close(ctx))
	}()
	require.NoError(t, cur.All(ctx, &results))

	assert.Len(t, results, controller.BatchSize*controller.Iterations)
	for _, result := range results {
		assert.NotNil(t, result.Migration.StartedAt)
		assert.NotNil(t, result.Migration.CompletedAt)
		assert.Equal(t, controller.Version, result.Migration.Version)
	}

	// Inject stale docs.
	numStaleDocs := numDocs / 2
	controller.Timeout = time.Hour
	updateController(t, ctx, env, controller)
	startedAt := time.Now().Add(-2 * controller.Timeout)
	updateMigrationStats(t, ctx, env, controller.Collection, numStaleDocs, model.MigrationStats{
		MigratorID: "stale-docs",
		StartedAt:  &startedAt,
	})

	j3 := NewBSONToParquetBackfillJob("some-stale")
	j3.Run(ctx)
	require.NoError(t, j3.Error())

	cur, err = env.GetDB().Collection(controller.Collection).Find(
		ctx,
		bson.M{bsonutil.GetDottedKeyName(model.TestResultsMigrationKey, model.MigrationStatsMigratorIDKey): j3.ID()},
	)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, cur.Close(ctx))
	}()
	require.NoError(t, cur.All(ctx, &results))

	assert.Len(t, results, numStaleDocs)
	for _, result := range results {
		assert.NotNil(t, result.Migration.StartedAt)
		assert.NotNil(t, result.Migration.CompletedAt)
		assert.Equal(t, controller.Version, result.Migration.Version)
	}
}

func updateController(t *testing.T, ctx context.Context, env cedar.Environment, controller model.BatchJobController) {
	_, err := env.GetDB().Collection(model.BatchJobControllerCollection).UpdateOne(
		ctx,
		bson.M{"_id": controller.ID},
		bson.M{"$set": &controller},
		options.Update().SetUpsert(true),
	)
	require.NoError(t, err)
}

func populateTestResults(t *testing.T, ctx context.Context, env cedar.Environment, tempCollection string, numDocs int) {
	for i := 0; i < numDocs; i++ {
		tr := getTestResults()
		tr.Setup(env)
		require.NoError(t, tr.SaveNew(ctx))
		_, err := env.GetDB().Collection(tempCollection).InsertOne(ctx, tr)
		require.NoError(t, err)
		results := make([]model.TestResult, 100)
		for j := 0; j < len(results); j++ {
			results[j] = getTestResult()
			results[j].TaskID = tr.Info.TaskID
			results[j].Execution = tr.Info.Execution
		}
		require.NoError(t, tr.Append(ctx, results))
	}
}

func updateMigrationStats(t *testing.T, ctx context.Context, env cedar.Environment, tempCollection string, numDocs int, stats model.MigrationStats) {
	cur, err := env.GetDB().Collection(tempCollection).Find(ctx, bson.M{}, options.Find().SetLimit(int64(numDocs)))
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, cur.Close(ctx))
	}()
	var results []model.TestResults
	require.NoError(t, cur.All(ctx, &results))
	require.Len(t, results, numDocs)

	for _, result := range results {
		_, err := env.GetDB().Collection(tempCollection).UpdateOne(
			ctx,
			bson.M{"_id": result.ID},
			bson.M{"$set": bson.M{model.TestResultsMigrationKey: &stats}},
		)
		require.NoError(t, err)
	}
}

func getTestResults() *model.TestResults {
	return model.CreateTestResults(
		model.TestResultsInfo{
			Project:                utility.RandomString(),
			Version:                utility.RandomString(),
			Variant:                utility.RandomString(),
			TaskName:               utility.RandomString(),
			TaskID:                 utility.RandomString(),
			Execution:              rand.Intn(5),
			RequestType:            utility.RandomString(),
			HistoricalDataDisabled: true,
		},
		model.PailLocal,
	)
}

func getTestResult() model.TestResult {
	return model.TestResult{
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
