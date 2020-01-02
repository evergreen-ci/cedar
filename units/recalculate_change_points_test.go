package units

import (
	"context"
	"strconv"
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

type TestResultsAndRollups struct {
	info    *model.PerformanceResultInfo
	rollups []model.PerfRollupValue
}

func makePerfResults(length int, breakpoint int, unique string) []TestResultsAndRollups {
	var rollups []TestResultsAndRollups
	i := 0
	for i < length {
		newRollup := TestResultsAndRollups{
			info: &model.PerformanceResultInfo{
				Project:  "project" + unique,
				Variant:  "variant",
				Version:  "version" + strconv.Itoa(i+1),
				Order:    i + 1,
				TestName: "test",
				TaskName: "task",
				Mainline: true,
			},
			rollups: []model.PerfRollupValue{
				{
					Name:       "measurement",
					MetricType: model.MetricTypeSum,
					Version:    0,
				},
				{
					Name:       "measurement_another",
					MetricType: model.MetricTypeSum,
					Version:    0,
				},
			},
		}
		if i < breakpoint {
			newRollup.rollups[0].Value = float64(100)
			newRollup.rollups[1].Value = float64(100)
		} else {
			newRollup.rollups[0].Value = float64(1000)
			newRollup.rollups[1].Value = float64(1000)
		}
		rollups = append(rollups, newRollup)
		i++
	}
	return rollups
}

func init() {
	dbName := "test_cedar_signal_processing"
	env, err := cedar.NewEnvironment(context.Background(), dbName, &cedar.Configuration{
		MongoDBURI:    "mongodb://localhost:27017",
		DatabaseName:  dbName,
		SocketTimeout: time.Minute,
		NumWorkers:    2,
	})
	if err != nil {
		panic(err)
	}
	cedar.SetEnvironment(env)
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

func (m *MockDetector) DetectChanges(ctx context.Context, series []float64) ([]perf.ChangePoint, error) {
	m.Calls = append(m.Calls, series)
	last := series[0]
	var cps []int
	for idx, i := range series {
		if i != last {
			last = i
			cps = append(cps, idx)
		}
	}
	var changePoints []perf.ChangePoint
	for _, cp := range cps {
		changePoints = append(changePoints, perf.ChangePoint{
			Index: cp,
			Algorithm: model.AlgorithmInfo{
				Name:    "some_algorithm",
				Version: 1,
				Options: []model.AlgorithmOption{
					{
						Name:  "some_option",
						Value: 5,
					},
					{
						Name:  "another_option",
						Value: 0.05,
					},
				},
			},
		})
	}

	return changePoints, nil
}

func TestRecalculateChangePointsJob(t *testing.T) {
	env := cedar.GetEnvironment()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = env.GetDB().Drop(ctx)

	rollups := makePerfResults(100, 50, "a")
	for _, x := range makePerfResults(100, 50, "b") {
		rollups = append(rollups, x)
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
	defer func() {
		assert.NoError(t, tearDown(env))
	}()

	t.Run("Recalculates", func(t *testing.T) {
		j := NewRecalculateChangePointsJob(model.TimeSeriesId{
			Project:     "projecta",
			Variant:     "variant",
			Task:        "task",
			Test:        "test",
			Measurement: "measurement",
		})
		mockDetector := &MockDetector{}
		j.(*RecalculateChangePointsJob).ChangePointDetector = mockDetector
		j.Run(ctx)
		assert.True(t, j.Status().Completed)
		assert.Len(t, mockDetector.Calls, 1)
		var result []model.PerformanceResult
		filter := bson.M{
			"change_points": bson.M{"$ne": []struct{}{}},
		}
		res, err := env.GetDB().Collection("perf_results").Find(ctx, filter)
		require.NoError(t, err)
		assert.NoError(t, res.All(ctx, &result))
		require.Len(t, result, 1)
		require.Len(t, result[0].ChangePoints, 1)
		require.Equal(t, result[0].ChangePoints[0].Measurement, "measurement")
		require.Equal(t, result[0].ChangePoints[0].Algorithm, model.AlgorithmInfo{
			Name:    "some_algorithm",
			Version: 1,
			Options: []model.AlgorithmOption{
				{
					Name:  "some_option",
					Value: int32(5),
				},
				{
					Name:  "another_option",
					Value: 0.05,
				},
			},
		})
	})

	t.Run("DoesNothingWhenDisabled", func(t *testing.T) {
		j := NewRecalculateChangePointsJob(model.TimeSeriesId{
			Project:     "projecta",
			Variant:     "variant",
			Task:        "task",
			Test:        "test",
			Measurement: "measurement",
		})
		mockDetector := &MockDetector{}
		job := j.(*RecalculateChangePointsJob)
		job.ChangePointDetector = mockDetector
		job.conf = model.NewCedarConfig(env)
		job.conf.Flags.DisableSignalProcessing = true
		j.Run(ctx)
		assert.True(t, j.Status().Completed)
		assert.Len(t, mockDetector.Calls, 0)
	})
}
