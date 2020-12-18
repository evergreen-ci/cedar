package units

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/perf"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

type testResultsAndRollups struct {
	info    *model.PerformanceResultInfo
	rollups []model.PerfRollupValue
}

func generateDistinctRandoms(existing []int, min, max, num int) []int {
	var newVals []int
	exists := map[int]bool{}
	for _, v := range existing {
		exists[v] = true
	}
	randoms := rand.Perm(max - min + 1)
	for _, v := range randoms {
		v += min
		if !exists[v] {
			exists[v] = true
			newVals = append(newVals, v)
			if len(newVals) == num {
				break
			}
		}
	}
	return newVals
}

func makePerfResultsWithChangePoints(unique string, seed int64) ([]testResultsAndRollups, [][]int) {
	// Deterministic testing on failure.
	grip.Debug("Seed for recalculate test: " + strconv.FormatInt(seed, 10))
	rand.Seed(seed)
	numTimeSeries := rand.Intn(10) + 1
	timeSeriesLengths := make([]int, numTimeSeries)
	timeSeriesChangePoints := make([][]int, numTimeSeries)
	timeSeries := make([][]int, numTimeSeries)

	// Generate series of random length in range [100, 300].
	for i := 0; i < numTimeSeries; i++ {
		timeSeriesLengths[i] = rand.Intn(201) + 100
	}

	// Sprinkle in some random change points.
	for measurement, length := range timeSeriesLengths {
		remainingPoints := length
		for remainingPoints > 0 {
			if remainingPoints < 30 {
				remainingPoints = 0
				continue
			}
			// Make sure there are 10 points on either side of the
			// change point.
			changePoint := rand.Intn(remainingPoints-20) + 10
			timeSeriesChangePoints[measurement] = append(timeSeriesChangePoints[measurement], changePoint+length-remainingPoints)
			remainingPoints = remainingPoints - changePoint
		}
	}

	// Create the time series.
	for measurement, length := range timeSeriesLengths {
		changePoints := timeSeriesChangePoints[measurement]
		startingPoint := 0
		stableValue := 1001
		for _, cp := range changePoints {
			stableValue = generateDistinctRandoms([]int{stableValue}, 0, 1000, 1)[0]
			for i := startingPoint; i < cp; i++ {
				timeSeries[measurement] = append(timeSeries[measurement], stableValue)
			}
			startingPoint = cp
		}
		stableValue = generateDistinctRandoms([]int{stableValue}, 0, 1000, 1)[0]
		for i := startingPoint; i < length; i++ {
			timeSeries[measurement] = append(timeSeries[measurement], stableValue)
		}
	}

	// Measurements/rollups can be added/removed over time, so we should
	// chop up and group our time series randomly.
	consumed := make([]int, numTimeSeries)
	var finishedConsuming []int
	var rollups []testResultsAndRollups

	i := 0
	measurementCount := 0
	for len(finishedConsuming) != numTimeSeries {
		// Let's record a random number of measurements this run, drawn
		// from [1, numTimeSeries-len(finishedConsuming)].
		measurementsThisRun := rand.Intn(numTimeSeries-len(finishedConsuming)) + 1
		// Get measurements that aren't yet finished being persisted.
		measurements := generateDistinctRandoms(finishedConsuming, 0, numTimeSeries-1, measurementsThisRun)
		measurementCount += len(measurements)

		newRollup := testResultsAndRollups{
			info: &model.PerformanceResultInfo{
				Project:   "project" + unique,
				Variant:   "variant",
				Version:   "version" + strconv.Itoa(i+1),
				Order:     i + 1,
				TestName:  "test",
				TaskName:  "task",
				TaskID:    fmt.Sprintf("task_id_%d", i),
				Execution: 1,
				Mainline:  true,
			},
		}

		for _, measurement := range measurements {
			newRollup.rollups = append(newRollup.rollups, model.PerfRollupValue{
				Name:       "measurement_" + strconv.Itoa(measurement),
				Value:      timeSeries[measurement][consumed[measurement]],
				Version:    0,
				MetricType: "sum",
			})
			consumed[measurement]++
			if consumed[measurement] == timeSeriesLengths[measurement] {
				finishedConsuming = append(finishedConsuming, measurement)
			}
		}
		rollups = append(rollups, newRollup)

		// Add the same rollup, but with a lesser execution. This
		// should be ignored.
		similarRollup := testResultsAndRollups{
			info: &model.PerformanceResultInfo{
				Project:   newRollup.info.Project,
				Variant:   newRollup.info.Variant,
				Version:   newRollup.info.Version,
				Order:     newRollup.info.Order,
				TestName:  newRollup.info.TestName,
				TaskName:  newRollup.info.TaskName,
				TaskID:    newRollup.info.TaskID,
				Execution: 0,
				Mainline:  newRollup.info.Mainline,
			},
			rollups: make([]model.PerfRollupValue, len(newRollup.rollups)),
		}
		copy(similarRollup.rollups, newRollup.rollups)
		rollups = append(rollups, similarRollup)

		i++
	}

	return rollups, timeSeries
}

func setupChangePointsTest() {
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

type MockPerformanceAnalysisService struct {
	Calls   []perf.TimeSeriesModel
	Results [][]int
}

func (m *MockPerformanceAnalysisService) ReportUpdatedTimeSeries(ctx context.Context, timeSeries perf.TimeSeriesModel) error {
	m.Calls = append(m.Calls, timeSeries)
	return nil
}

func provisionDb(ctx context.Context, env cedar.Environment, rollups []testResultsAndRollups) {
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

func TestRecalculateChangePointsJob(t *testing.T) {
	setupChangePointsTest()
	env := cedar.GetEnvironment()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	defer func() {
		require.NoError(t, tearDown(env))
	}()

	t.Run("ReportsTimeSeries", func(t *testing.T) {
		_ = env.GetDB().Drop(ctx)
		rollups, timeSeries := makePerfResultsWithChangePoints("a", time.Now().UnixNano())
		provisionDb(ctx, env, rollups)

		timeSeriesId := model.PerformanceResultSeriesID{
			Project:   "projecta",
			Variant:   "variant",
			Task:      "task",
			Test:      "test",
			Arguments: map[string]int32{},
		}
		j := NewUpdateTimeSeriesJob(timeSeriesId)
		mockDetector := &MockPerformanceAnalysisService{}
		job := j.(*timeSeriesUpdateJob)
		job.performanceAnalysisService = mockDetector
		job.conf = model.NewCedarConfig(env)
		j.Run(ctx)
		require.True(t, j.Status().Completed)
		require.NoError(t, j.Error())
		require.Equal(t, len(timeSeries), len(mockDetector.Calls))
		for _, call := range mockDetector.Calls {
			timeSeriesIndex, _ := strconv.Atoi(string(call.Measurement[len(call.Measurement)-1]))
			require.Equal(t, timeSeriesId.Project, call.Project)
			require.Equal(t, timeSeriesId.Variant, call.Variant)
			require.Equal(t, timeSeriesId.Task, call.Task)
			require.Equal(t, timeSeriesId.Test, call.Test)
			require.Equal(t, len(timeSeriesId.Arguments), len(call.Arguments))
			data := make([]int, len(call.Data))
			for i, timeSeriesData := range call.Data {
				data[i] = int(timeSeriesData.Value)
			}
			require.Equal(t, timeSeries[timeSeriesIndex], data)
		}
	})

	t.Run("DoesNothingWhenDisabled", func(t *testing.T) {
		j := NewUpdateTimeSeriesJob(model.PerformanceResultSeriesID{
			Project: "projecta",
			Variant: "variant",
			Task:    "task",
			Test:    "test",
		})
		mockDetector := &MockPerformanceAnalysisService{}
		job := j.(*timeSeriesUpdateJob)
		job.performanceAnalysisService = mockDetector
		job.conf = model.NewCedarConfig(env)
		job.conf.Flags.DisableSignalProcessing = true
		j.Run(ctx)
		require.True(t, j.Status().Completed)
		require.Len(t, mockDetector.Calls, 0)
	})
}
