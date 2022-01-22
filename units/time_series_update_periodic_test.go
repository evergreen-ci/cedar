package units

import (
	"context"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/mongodb/amboy/queue"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
)

func setupPeriodic() {
	dbName := "test_cedar_signal_processing_periodic"
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

func tearDownPeriodicTest(env cedar.Environment) error {
	conf, session, err := cedar.GetSessionWithConfig(env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()
	return errors.WithStack(session.DB(conf.DatabaseName).DropDatabase())
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
				Project:   "project" + unique,
				Variant:   "variant",
				Version:   "version" + strconv.Itoa(i+1),
				Order:     i + 1,
				TestName:  "test",
				TaskName:  "task",
				Arguments: map[string]int32{"thread_level": 20},
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
		i++
	}

	return rollups, timeSeries
}

func TestPeriodicTimeSeriesUpdateJob(t *testing.T) {
	setupPeriodic()
	env := cedar.GetEnvironment()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, tearDownPeriodicTest(env))
	}()

	t.Run("PeriodicallySchedules", func(t *testing.T) {
		_ = env.GetDB().Drop(ctx)

		aRollups, _ := makePerfResultsWithChangePoints("e", time.Now().UnixNano())
		bRollups, _ := makePerfResultsWithChangePoints("f", time.Now().UnixNano())
		provisionDb(ctx, env, append(aRollups, bRollups...))

		j := NewPeriodicTimeSeriesUpdateJob("someId")
		job := j.(*periodicTimeSeriesJob)
		job.queue = queue.NewLocalLimitedSize(1, 100)
		assert.NoError(t, job.queue.Start(ctx))
		j.Run(ctx)
		require.NoError(t, j.Error())
		assert.True(t, j.Status().Completed)
		assert.Equal(t, 2, job.queue.Stats(ctx).Total)

	})
}
