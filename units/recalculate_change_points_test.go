package units

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar/perf"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/mgo.v2/bson"
)

type TestResultsAndRollups []struct {
	info    *model.PerformanceResultInfo
	rollups []model.PerfRollupValue
}

func init() {
	dbName := "cedar_signal_processing"
	env, err := cedar.NewEnvironment(context.Background(), dbName, &cedar.Configuration{
		MongoDBURI:    "mongodb://localhost:27017",
		DatabaseName:  dbName,
		SocketTimeout: time.Minute,
		NumWorkers:    2,
	})
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	cedar.SetEnvironment(env)

	rollups := TestResultsAndRollups{
		{
			info: &model.PerformanceResultInfo{
				Project:  "project1",
				Variant:  "variant1",
				Version:  "0",
				Order:    1,
				TestName: "test1",
				TaskName: "task1",
				TaskID:   "task1",
				Mainline: true,
			},
			rollups: []model.PerfRollupValue{
				{
					Name:       "OverheadTotal",
					MetricType: model.MetricTypeSum,
					Version:    0,
					Value:      100,
				},
				{
					Name:       "OperationsTotal",
					MetricType: model.MetricTypeSum,
					Version:    0,
					Value:      10000,
				},
			},
		},
		{
			info: &model.PerformanceResultInfo{
				Project:  "project2",
				Variant:  "variant1",
				Version:  "0",
				Order:    1,
				TestName: "test1",
				TaskName: "task1",
				TaskID:   "task1",
				Mainline: true,
			},
			rollups: []model.PerfRollupValue{
				{
					Name:       "OperationsTotal",
					MetricType: model.MetricTypeSum,
					Version:    0,
					Value:      10000,
				},
			},
		},
		{
			info: &model.PerformanceResultInfo{
				Project:  "project1",
				Variant:  "variant1",
				Version:  "1",
				Order:    2,
				TestName: "test1",
				TaskName: "task1",
				TaskID:   "task1",
				Mainline: true,
			},
			rollups: []model.PerfRollupValue{
				{
					Name:       "OverheadTotal",
					MetricType: model.MetricTypeSum,
					Version:    0,
					Value:      100,
				},
				{
					Name:       "OperationsTotal",
					MetricType: model.MetricTypeSum,
					Version:    0,
					Value:      10000,
				},
			},
		},
	}

	for _, result := range rollups {
		performanceResult := model.CreatePerformanceResult(*result.info, nil, result.rollups)
		performanceResult.CreatedAt = time.Now().Add(time.Second * -1)
		performanceResult.Setup(env)
		err := performanceResult.SaveNew(ctx)
		if err != nil {
			panic(err)
		}
	}
}

func tearDown(env cedar.Environment) error {
	conf, session, err := cedar.GetSessionWithConfig(env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()
	return errors.WithStack(session.DB(conf.DatabaseName).DropDatabase())
}

type MockDetector struct {
	Calls [][]float64
}

func (m *MockDetector) DetectChanges(context.Context, []float64) ([]perf.ChangePoint, error) {
	panic("implement me")
}

func TestRecalculateChangePointsJob(t *testing.T) {
	if runtime.GOOS == "darwin" && os.Getenv("EVR_TASK_ID") != "" {
		t.Skip("avoid less relevant failing test in evergreen")
	}

	env := cedar.GetEnvironment()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	defer func() {
		assert.NoError(t, tearDown(env))
	}()

	t.Run("ValidData", func(t *testing.T) {
		j := NewRecalculateChangePointsJob(model.TimeSeriesId{
			Project:     "",
			Variant:     "",
			Task:        "",
			Test:        "",
			Measurement: "",
		})
		j.(*RecalculateChangePointsJob).ChangePointDetector = &MockDetector{}
		j.Run(ctx)
		assert.True(t, j.Status().Completed)
		result := &model.PerformanceResult{}
		res := env.GetDB().Collection("perf_results").FindOne(ctx, bson.M{"_id": validResult.ID})
		require.NoError(t, res.Err())
		assert.NoError(t, res.Decode(result))
		require.NotNil(t, result.Rollups)
		assert.True(t, len(j.RollupTypes) <= len(result.Rollups.Stats))
		assert.Zero(t, result.FailedRollupAttempts)
		for _, stats := range result.Rollups.Stats {
			assert.Equal(t, user, stats.UserSubmitted)
		}
	})
}
