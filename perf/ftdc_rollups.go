package perf

import (
	"time"

	"github.com/evergreen-ci/cedar/model"
	"github.com/mongodb/ftdc"
	"github.com/pkg/errors"
)

const maxDurationsSize = 250000000

type PerformanceStatistics struct {
	counters struct {
		operationsTotal int64
		documentsTotal  int64
		sizeTotal       int64
		errorsTotal     int64
	}

	timers struct {
		extractedDurations []float64
		durationTotal      time.Duration
		total              time.Duration
		totalWallTime      time.Duration
	}

	gauges struct {
		state   []float64
		workers []float64
		failed  []float64
	}
}

func CalculateDefaultRollups(dx *ftdc.ChunkIterator, user bool) ([]model.PerfRollupValue, error) {
	rollups := []model.PerfRollupValue{}

	perfStats, err := CreatePerformanceStats(dx)
	if err != nil {
		return rollups, errors.Wrap(err, "problem calculating perf statistics")
	}

	factories := DefaultRollupFactories()
	for _, factory := range factories {
		rollups = append(rollups, factory.Calc(perfStats, user)...)
	}

	return rollups, nil
}

func CreatePerformanceStats(dx *ftdc.ChunkIterator) (*PerformanceStatistics, error) {
	perfStats := &PerformanceStatistics{}
	lastValue := float64(0)
	var start time.Time
	var end time.Time

	defer dx.Close()
	for i := 0; dx.Next(); i++ {
		chunk := dx.Chunk()

		for _, metric := range chunk.Metrics {
			switch name := metric.Key(); name {
			case "counters.ops":
				perfStats.counters.operationsTotal = metric.Values[len(metric.Values)-1]
			case "counters.n":
				perfStats.counters.documentsTotal = metric.Values[len(metric.Values)-1]
			case "counters.size":
				perfStats.counters.sizeTotal = metric.Values[len(metric.Values)-1]
			case "counters.errors":
				perfStats.counters.errorsTotal = metric.Values[len(metric.Values)-1]
			case "timers.duration", "timers.dur":
				perfStats.timers.extractedDurations = append(
					perfStats.timers.extractedDurations,
					extractValues(convertToFloats(metric.Values), lastValue)...,
				)
				// In order to avoid memory panics, reject
				// anything larger than 2GB.
				if len(perfStats.timers.extractedDurations) > maxDurationsSize {
					return nil, errors.New("size of ftdc file exceeds 2GB")
				}
				lastValue = float64(metric.Values[len(metric.Values)-1])
				perfStats.timers.durationTotal = time.Duration(metric.Values[len(metric.Values)-1])
			case "timers.total":
				perfStats.timers.total = time.Duration(metric.Values[len(metric.Values)-1])
			case "gauges.state":
				perfStats.gauges.state = convertToFloats(metric.Values)
			case "gauges.workers":
				perfStats.gauges.workers = convertToFloats(metric.Values)
			case "gauges.failed":
				perfStats.gauges.failed = convertToFloats(metric.Values)
			case "ts":
				if i == 0 {
					t := metric.Values[0]
					start = time.Unix(t/1000, t%1000*1000000)
				}
				t := metric.Values[len(metric.Values)-1]
				end = time.Unix(t/1000, t%1000*1000000)
			case "id":
				continue
			default:
				return nil, errors.Errorf("unknown field name %s", name)
			}
		}
	}

	perfStats.timers.totalWallTime = end.Sub(start)

	return perfStats, errors.WithStack(dx.Err())
}

func convertToFloats(ints []int64) []float64 {
	floats := []float64{}
	for i := range ints {
		floats = append(floats, float64(ints[i]))
	}

	return floats
}

// expects slice of cumulative values
func extractValues(vals []float64, lastValue float64) []float64 {
	extractedVals := make([]float64, len(vals))

	for i := range vals {
		extractedVals[i] = vals[i] - lastValue
		lastValue = vals[i]
	}

	return extractedVals
}
