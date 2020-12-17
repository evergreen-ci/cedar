package units

import (
	"context"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	for testName, testCase := range map[string]func(ctx context.Context, t *testing.T, env cedar.Environment){
		"CreatesNewHistoricalTestData": func(ctx context.Context, t *testing.T, env cedar.Environment) {
			info := makeInfo()
			tr := makeResult()
			j := makeJob(t, env, info, tr)

			j.Run(ctx)
			// It will error because it cannot query the Evergreen
			// API, but the job should still continue.
			assert.NotZero(t, j.Error())

			htd, err := model.CreateHistoricalTestData(j.Info)
			require.NoError(t, err)
			htd.Setup(env)
			require.NoError(t, htd.Find(ctx))

			dur := tr.TestEndTime.Sub(tr.TestStartTime)
			assert.EqualValues(t, dur, htd.AverageDuration)
			assert.Equal(t, 1, htd.NumPass)
			assert.Zero(t, htd.NumFail)
			assert.WithinDuration(t, htd.LastUpdate, time.Now(), time.Minute)

			require.NoError(t, htd.Remove(ctx))
		},
		"UpdatesExistingHistoricalTestData": func(ctx context.Context, t *testing.T, env cedar.Environment) {
			info := makeInfo()
			tr := makeResult()

			j := makeJob(t, env, info, tr)

			htd, err := model.CreateHistoricalTestData(j.Info)
			require.NoError(t, err)
			htd.Setup(env)
			require.NoError(t, htd.Update(ctx, tr))

			j.Run(ctx)
			// It will error because it cannot query the Evergreen
			// API, but the job should still continue.
			assert.NotZero(t, j.Error())

			htd = &model.HistoricalTestData{ID: htd.ID}
			htd.Setup(env)
			require.NoError(t, htd.Find(ctx))

			dur := tr.TestEndTime.Sub(tr.TestStartTime)
			assert.EqualValues(t, dur, htd.AverageDuration)
			assert.Equal(t, 2, htd.NumPass)
			assert.Zero(t, htd.NumFail)
			assert.WithinDuration(t, htd.LastUpdate, time.Now(), time.Minute)

			require.NoError(t, htd.Remove(ctx))
		},
	} {
		t.Run(testName, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			conf := model.NewCedarConfig(env)
			require.NoError(t, conf.Save())
			env := cedar.GetEnvironment()
			testCase(ctx, t, env)
		})
	}
}
