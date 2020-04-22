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

func makePerfResultsWithChangePoints(unique string, seed int64) ([]testResultsAndRollups, map[string][]int) {
	// deterministic testing on failure
	fmt.Println("Seed for recalculate test: " + strconv.FormatInt(seed, 10))
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
	changePoints := map[string][]int{}
	for measurement, cpIndices := range timeSeriesChangePoints {
		measurementName := "measurement_" + strconv.Itoa(measurement)
		changePoints[measurementName] = append(changePoints[measurementName], cpIndices...)
	}

	return rollups, changePoints
}

func setup() {
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
	Calls   [][]float64
	Results [][]int
}

func (m *MockDetector) Algorithm() perf.Algorithm {
	return perf.CreateDefaultAlgorithm()
}

func (m *MockDetector) DetectChanges(ctx context.Context, series []float64) ([]int, error) {
	m.Calls = append(m.Calls, series)
	last := series[0]
	cps := []int{}
	for idx, i := range series {
		if i != last {
			last = i
			cps = append(cps, idx)
		}
	}
	m.Results = append(m.Results, cps)
	return cps, nil
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

func extractAndValidateChangePointsFromDb(ctx context.Context, env cedar.Environment, t *testing.T, detector perf.ChangeDetector) map[string]map[string][]int {
	result := getPerformanceResultsWithChangePoints(ctx, env, t)

	var options []model.AlgorithmOption
	for _, v := range detector.Algorithm().Configuration() {
		options = append(options, model.AlgorithmOption{
			Name:  v.Name,
			Value: v.Value,
		})
	}

	createdChangePoints := map[string]map[string][]int{}
	for _, v := range result {
		// make sure processed_at got set in db
		require.NotEqual(t, v.Analysis.ProcessedAt, time.Time{})

		if createdChangePoints[v.Info.Project] == nil {
			createdChangePoints[v.Info.Project] = map[string][]int{}
		}
		for _, cp := range v.Analysis.ChangePoints {
			// make sure all the algorithm and triage info is correct
			require.Equal(t, cp.Algorithm.Name, detector.Algorithm().Name())
			require.Equal(t, cp.Algorithm.Version, detector.Algorithm().Version())
			for _, o := range options {
				require.Contains(t, cp.Algorithm.Options, o)
			}
			require.Equal(t, cp.Triage.TriagedOn, time.Time{})
			require.Equal(t, cp.Triage.Status, model.TriageStatusUntriaged)
			createdChangePoints[v.Info.Project][cp.Measurement] = append(createdChangePoints[v.Info.Project][cp.Measurement], cp.Index)
		}
	}

	return createdChangePoints
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
	setup()
	env := cedar.GetEnvironment()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	defer func() {
		require.NoError(t, tearDown(env))
	}()

	t.Run("Recalculates", func(t *testing.T) {
		_ = env.GetDB().Drop(ctx)

		aRollups, aChangePoints := makePerfResultsWithChangePoints("a", time.Now().UnixNano())
		bRollups, bChangePoints := makePerfResultsWithChangePoints("b", time.Now().UnixNano())
		provisionDb(ctx, env, append(aRollups, bRollups...))

		j := NewRecalculateChangePointsJob(model.PerformanceResultSeriesID{
			Project: "projecta",
			Variant: "variant",
			Task:    "task",
			Test:    "test",
		})
		mockDetector := &MockDetector{}
		j.(*recalculateChangePointsJob).changePointDetector = mockDetector
		j.Run(ctx)

		// Check that we made one call for each measurement/rollup
		require.Len(t, mockDetector.Calls, len(aChangePoints))

		createdChangePoints := extractAndValidateChangePointsFromDb(ctx, env, t, mockDetector)

		// make sure everything is where we think it should be
		require.Equal(t, aChangePoints, createdChangePoints["projecta"])

		// make sure we only calculated cps for projecta
		require.Len(t, createdChangePoints, 1)

		// Now let's calculate for another project
		j = NewRecalculateChangePointsJob(model.PerformanceResultSeriesID{
			Project: "projectb",
			Variant: "variant",
			Task:    "task",
			Test:    "test",
		})
		j.(*recalculateChangePointsJob).changePointDetector = mockDetector
		j.Run(ctx)
		require.True(t, j.Status().Completed)

		// Check that we made one call for each measurement/rollup
		require.Len(t, mockDetector.Calls, len(aChangePoints)+len(bChangePoints))

		createdChangePoints = extractAndValidateChangePointsFromDb(ctx, env, t, mockDetector)

		// make sure everything is where we think it should be
		require.Equal(t, aChangePoints, createdChangePoints["projecta"])
		require.Equal(t, bChangePoints, createdChangePoints["projectb"])

		// make sure we only calculated cps for projecta & project b
		require.Len(t, createdChangePoints, 2)
	})

	t.Run("IgnoresHistoryBeforeTriagedChangePoint", func(t *testing.T) {
		setup()
		_ = env.GetDB().Drop(ctx)

		cRollups, _ := makePerfResultsWithChangePoints("c", time.Now().UnixNano())
		provisionDb(ctx, env, cRollups)

		j := NewRecalculateChangePointsJob(model.PerformanceResultSeriesID{
			Project: "projectc",
			Variant: "variant",
			Task:    "task",
			Test:    "test",
		})
		mockDetector := &MockDetector{}
		j.(*recalculateChangePointsJob).changePointDetector = mockDetector
		j.Run(ctx)

		performanceResults := getPerformanceResultsWithChangePoints(ctx, env, t)

		for _, perfResult := range performanceResults {
			for _, cp := range perfResult.Analysis.ChangePoints {
				// let's triage the cps
				_ = model.TriageChangePoint(ctx, env, perfResult.ID, cp.Measurement, model.TriageStatusTruePositive)
			}
		}

		performanceResults = getPerformanceResultsWithChangePoints(ctx, env, t)
		var originalChangePoints []model.ChangePoint
		for _, perfResult := range performanceResults {
			originalChangePoints = append(originalChangePoints, perfResult.Analysis.ChangePoints...)
		}

		j = NewRecalculateChangePointsJob(model.PerformanceResultSeriesID{
			Project: "projectc",
			Variant: "variant",
			Task:    "task",
			Test:    "test",
		})
		mockDetector = &MockDetector{}
		j.(*recalculateChangePointsJob).changePointDetector = mockDetector
		j.Run(ctx)

		for _, result := range mockDetector.Results {
			// check that we're not detecting anything new.
			require.Equal(t, result, []int{})
		}

		performanceResults = getPerformanceResultsWithChangePoints(ctx, env, t)
		var newChangePoints []model.ChangePoint
		for _, perfResult := range performanceResults {
			newChangePoints = append(newChangePoints, perfResult.Analysis.ChangePoints...)
		}

		// make sure all the original change points are still there (not evicted)
		require.Equal(t, originalChangePoints, newChangePoints)
	})

	t.Run("DoesNothingWhenDisabled", func(t *testing.T) {
		j := NewRecalculateChangePointsJob(model.PerformanceResultSeriesID{
			Project: "projecta",
			Variant: "variant",
			Task:    "task",
			Test:    "test",
		})
		mockDetector := &MockDetector{}
		job := j.(*recalculateChangePointsJob)
		job.changePointDetector = mockDetector
		job.conf = model.NewCedarConfig(env)
		job.conf.Flags.DisableSignalProcessing = true
		j.Run(ctx)
		require.True(t, j.Status().Completed)
		require.Len(t, mockDetector.Calls, 0)
	})
}
