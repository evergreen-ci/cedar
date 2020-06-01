package units

import (
	"context"
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
	"gopkg.in/mgo.v2/bson"
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

type changePointIndexPercentChange struct {
	index         int
	percentChange float64
}

func makePerfResultsWithChangePoints(unique string, seed int64) ([]testResultsAndRollups, map[string][]changePointIndexPercentChange) {
	// deterministic testing on failure
	grip.Debug("Seed for recalculate test: " + strconv.FormatInt(seed, 10))
	rand.Seed(seed)
	numTimeSeries := rand.Intn(10) + 1
	timeSeriesLengths := make([]int, numTimeSeries)
	timeSeriesChangePoints := make([][]int, numTimeSeries)
	timeSeries := make([][]int, numTimeSeries)

	// generate series of random length in range [100, 300]
	for i := 0; i < numTimeSeries; i++ {
		timeSeriesLengths[i] = rand.Intn(201) + 100
	}

	// sprinkle in some random change points
	for measurement, length := range timeSeriesLengths {
		remainingPoints := length
		for remainingPoints > 0 {
			if remainingPoints < 30 {
				remainingPoints = 0
				continue
			}
			// make sure there are 10 points on either side of the cp
			changePoint := rand.Intn(remainingPoints-20) + 10
			timeSeriesChangePoints[measurement] = append(timeSeriesChangePoints[measurement], changePoint+length-remainingPoints)
			remainingPoints = remainingPoints - changePoint
		}
	}

	// let's create our time series
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

	// measurements/rollups can be added/removed over time, so we should chop up and group our time series randomly
	consumed := make([]int, numTimeSeries)
	var finishedConsuming []int
	var rollups []testResultsAndRollups

	i := 0
	for len(finishedConsuming) != numTimeSeries {
		// Let's record a random number of measurements this run, drawn from [1, numTimeSeries-len(finishedConsuming)]
		measurementsThisRun := rand.Intn(numTimeSeries-len(finishedConsuming)) + 1
		// Get measurements that aren't yet finished being persisted
		measurements := generateDistinctRandoms(finishedConsuming, 0, numTimeSeries-1, measurementsThisRun)

		newRollup := testResultsAndRollups{
			info: &model.PerformanceResultInfo{
				Project:  "project" + unique,
				Variant:  "variant",
				Version:  "version" + strconv.Itoa(i+1),
				Order:    i + 1,
				TestName: "test",
				TaskName: "task",
				Mainline: true,
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
		i++
	}

	// time to map over change points
	changePoints := map[string][]changePointIndexPercentChange{}
	for measurement, cpIndices := range timeSeriesChangePoints {
		measurementName := "measurement_" + strconv.Itoa(measurement)
		cpsWithIndexAndPercentChange := make([]changePointIndexPercentChange, len(cpIndices))
		currentTimeSeries := timeSeries[measurement]
		for i, pointIndex := range cpIndices {
			// No need to average over windows, since they are constant --> just use points before/after the change
			percentChange := 100 * ((float64(currentTimeSeries[pointIndex]) / float64(currentTimeSeries[pointIndex-1])) - 1)
			cpsWithIndexAndPercentChange[i] = changePointIndexPercentChange{index: pointIndex, percentChange: percentChange}
		}

		changePoints[measurementName] = append(changePoints[measurementName], cpsWithIndexAndPercentChange...)
	}

	return rollups, changePoints
}

func convertIntArrayToFloat64(input []int) []float64 {
	floatArray := make([]float64, len(input))
	for i, curInt := range input {
		floatArray[i] = float64(curInt)
	}
	return floatArray
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

func getPerformanceResultsWithChangePoints(ctx context.Context, env cedar.Environment, t *testing.T) []model.PerformanceResult {
	// Get all perf results with change points
	var result []model.PerformanceResult
	filter := bson.M{
		"analysis.change_points": bson.M{"$ne": []struct{}{}},
	}
	res, err := env.GetDB().Collection("perf_results").Find(ctx, filter)
	require.NoError(t, err)
	require.NoError(t, res.All(ctx, &result))
	return result
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
		j := NewUpdateTimeSeriesJob(model.PerformanceResultSeriesID{
			Project:   "projecta",
			Variant:   "variant",
			Task:      "task",
			Test:      "test",
			Arguments: map[string]int32{},
		})
		mockDetector := perf.NewPerformanceAnalysisService("http://localhost:8080", "", "")
		job := j.(*timeSeriesUpdateJob)
		job.performanceAnalysisService = mockDetector
		job.conf = model.NewCedarConfig(env)
		j.Run(ctx)
		require.True(t, j.Status().Completed)
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
