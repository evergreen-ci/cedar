package units

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/pail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/mgo.v2/bson"
)

func setupEnv(ctx context.Context, t *testing.T) {
}

func TestHistoricalTestDataJob(t *testing.T) {
	dbName := "cedar-historical-test-data"
	env, err := cedar.NewEnvironment(context.Background(), dbName, &cedar.Configuration{
		MongoDBURI:              "mongodb://localhost:27017",
		DatabaseName:            dbName,
		SocketTimeout:           time.Minute,
		NumWorkers:              2,
		DisableLocalQueue:       true,
		DisableRemoteQueue:      true,
		DisableRemoteQueueGroup: true,
	})
	require.NoError(t, err)
	cedar.SetEnvironment(env)

	defer func() {
		conf, session, err := cedar.GetSessionWithConfig(env)
		require.NoError(t, err)
		assert.NoError(t, session.DB(conf.DatabaseName).DropDatabase())
		session.Close()
	}()

	makeInfo := func() model.TestResultsInfo {
		return model.TestResultsInfo{
			Project:     "project",
			Version:     "version",
			Variant:     "variant",
			TaskName:    "task_name",
			TaskID:      "task_id",
			Execution:   1,
			RequestType: "request_type",
		}
	}
	makeResult := func() model.TestResult {
		return model.TestResult{
			TaskID:         "task_id",
			Execution:      1,
			TestName:       "test_name",
			LineNum:        100,
			Status:         "pass",
			TaskCreateTime: time.Now().Add(-time.Hour),
			TestStartTime:  time.Now().Add(-30 * time.Minute),
			TestEndTime:    time.Now(),
		}
	}

	makeJob := func(t *testing.T, env cedar.Environment, info model.TestResultsInfo, tr model.TestResult) *historicalTestDataJob {
		j, ok := NewHistoricalTestDataJob(env, info, tr).(*historicalTestDataJob)
		require.True(t, ok)
		return j
	}

	for testName, testCase := range map[string]func(ctx context.Context, t *testing.T, env cedar.Environment, bucket pail.Bucket){
		"CreatesNewHistoricalTestData": func(ctx context.Context, t *testing.T, env cedar.Environment, bucket pail.Bucket) {
			info := makeInfo()
			tr := makeResult()
			j := makeJob(t, env, info, tr)

			j.Run(ctx)
			// It will error because it cannot query the Evergreen API, but the
			// job should still continue.
			assert.NotZero(t, j.Error())

			htd, err := model.CreateHistoricalTestData(j.Info, model.PailLocal)
			require.NoError(t, err)
			htdr, err := bucket.Get(ctx, htd.Path())
			require.NoError(t, err)
			b, err := ioutil.ReadAll(htdr)
			require.NoError(t, err)
			htd = &model.HistoricalTestData{}
			require.NoError(t, bson.Unmarshal(b, &htd))

			dur := tr.TestEndTime.Sub(tr.TestStartTime)
			assert.Equal(t, []time.Duration{dur}, htd.Durations)
			assert.Equal(t, dur, htd.AverageDuration)
			assert.Equal(t, 1, htd.NumPass)
			assert.Zero(t, htd.NumFail)
			assert.WithinDuration(t, htd.LastUpdate, time.Now(), time.Minute)
		},
		"UpdatesExistingHistoricalTestData": func(ctx context.Context, t *testing.T, env cedar.Environment, bucket pail.Bucket) {
			info := makeInfo()
			tr := makeResult()

			j := makeJob(t, env, info, tr)

			htd, err := model.CreateHistoricalTestData(j.Info, model.PailLocal)
			require.NoError(t, err)
			htd.Setup(env)
			htd.NumPass = 1
			dur := tr.TestEndTime.Sub(tr.TestStartTime)
			htd.Durations = []time.Duration{dur}
			htd.AverageDuration = dur
			require.NoError(t, htd.Save(ctx))

			j.Run(ctx)
			// It will error because it cannot query the Evergreen API, but the
			// job should still continue.
			assert.NotZero(t, j.Error())

			htdr, err := bucket.Get(ctx, htd.Path())
			require.NoError(t, err)
			b, err := ioutil.ReadAll(htdr)
			require.NoError(t, err)
			htd = &model.HistoricalTestData{}
			require.NoError(t, bson.Unmarshal(b, &htd))

			assert.Equal(t, []time.Duration{dur, dur}, htd.Durations)
			assert.Equal(t, dur, htd.AverageDuration)
			assert.Equal(t, 2, htd.NumPass)
			assert.Zero(t, htd.NumFail)
			assert.WithinDuration(t, htd.LastUpdate, time.Now(), time.Minute)

		},
	} {
		t.Run(testName, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			tmpDir, err := ioutil.TempDir("", "historical-test-data-job")
			require.NoError(t, err)
			defer func() {
				assert.NoError(t, os.RemoveAll(tmpDir))
			}()

			bucket, err := pail.NewLocalBucket(pail.LocalOptions{Path: tmpDir})
			require.NoError(t, err)

			env := cedar.GetEnvironment()
			conf := model.NewCedarConfig(env)
			conf.Bucket = model.BucketConfig{
				HistoricalTestDataBucket:     tmpDir,
				HistoricalTestDataBucketType: model.PailLocal,
			}
			require.NoError(t, conf.Save())
			testCase(ctx, t, env, bucket)
		})
	}
}
