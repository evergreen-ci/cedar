package perf

import (
	"math"
	"time"

	"github.com/evergreen-ci/sink/model"
	"github.com/evergreen-ci/sink/util"
	"github.com/mongodb/ftdc"
	"github.com/pkg/errors"
)

const (
	defaultVer = 1
)

type performanceStatistics struct {
	counters struct {
		operations int64
		size       int64
		errors     int64
	}

	timers struct {
		durationTotal time.Duration
		total         time.Duration
	}

	gauges struct {
		stateTotal   int64
		workersTotal int64
		failedTotal  int64
	}

	numSamples int
}

func CalculateDefaultRollups(dx *ftdc.ChunkIterator) ([]model.PerfRollupValue, error) {
	rollups := []model.PerfRollupValue{}

	perfStats, err := createPerformanceStats(dx)
	if err != nil {
		return rollups, errors.Wrap(err, "problem calculating perf rollups")
	}

	rollups = append(rollups, perfStats.perfMeans()...)
	rollups = append(rollups, perfStats.perfThroughputs()...)
	rollups = append(rollups, perfStats.perfLatencies()...)
	rollups = append(rollups, perfStats.perfTotals()...)
	return rollups, nil
}

func createPerformanceStats(dx *ftdc.ChunkIterator) (performanceStatistics, error) {
	perfStats := performanceStatistics{}

	defer dx.Close()
	for i := 0; dx.Next(); i++ {
		chunk := dx.Chunk()
		perfStats.numSamples += chunk.Size()

		for _, metric := range chunk.Metrics {
			switch name := metric.Key(); name {
			case "Counters.Operations":
				perfStats.counters.operations = metric.Values[len(metric.Values)-1]
			case "Counters.Size":
				perfStats.counters.size = metric.Values[len(metric.Values)-1]
			case "Counters.Errors":
				perfStats.counters.errors = metric.Values[len(metric.Values)-1]
			case "Timers.Duration":
				perfStats.timers.durationTotal += time.Duration(util.SumInt64(metric.Values))
			case "Timers.Total":
				perfStats.timers.total = time.Duration(metric.Values[len(metric.Values)-1])
			case "Gauges.State":
				perfStats.gauges.stateTotal += util.SumInt64(metric.Values)
			case "Gauges.Workers":
				perfStats.gauges.workersTotal += util.SumInt64(metric.Values)
			case "Gauges.Failed":
				perfStats.gauges.failedTotal += util.SumInt64(metric.Values)
			default:
				return performanceStatistics{}, errors.Errorf("unknown field name %s", name)
			}
		}
	}
	return perfStats, nil
}

func (s *performanceStatistics) perfMeans() []model.PerfRollupValue {
	rollups := []model.PerfRollupValue{}

	if s.numSamples > 0 {
		return append(
			rollups,
			model.PerfRollupValue{
				Name:          "avgDuration",
				Value:         float64(s.timers.durationTotal) / float64(s.numSamples),
				Version:       defaultVer,
				UserSubmitted: false,
			},
			model.PerfRollupValue{
				Name:          "avgState",
				Value:         float64(s.gauges.stateTotal) / float64(s.numSamples),
				Version:       defaultVer,
				UserSubmitted: false,
			},
			model.PerfRollupValue{
				Name:          "avgWorkers",
				Value:         float64(s.gauges.workersTotal) / float64(s.numSamples),
				Version:       defaultVer,
				UserSubmitted: false,
			},
		)
	}
	return rollups
}

func (s *performanceStatistics) perfThroughputs() []model.PerfRollupValue {
	rollups := []model.PerfRollupValue{}

	if s.timers.durationTotal > 0 {
		return append(
			rollups,
			model.PerfRollupValue{
				Name:          "throughputOps",
				Value:         float64(s.counters.operations) / float64(s.timers.durationTotal.Seconds()),
				Version:       defaultVer,
				UserSubmitted: false,
			},
			model.PerfRollupValue{
				Name:          "throughputSize",
				Value:         float64(s.counters.size) / float64(s.timers.durationTotal.Seconds()),
				Version:       defaultVer,
				UserSubmitted: false,
			},
			model.PerfRollupValue{
				Name:          "errorRate",
				Value:         float64(s.counters.errors) / float64(s.timers.durationTotal.Seconds()),
				Version:       defaultVer,
				UserSubmitted: false,
			},
		)
	}
	return rollups
}

func (s *performanceStatistics) perfLatencies() []model.PerfRollupValue {
	rollups := []model.PerfRollupValue{}

	var value float64
	if s.counters.operations == 0 {
		value = math.Inf(0)
	} else {
		value = float64(s.timers.durationTotal) / float64(s.counters.operations)
	}
	return append(rollups, model.PerfRollupValue{
		Name:          "latency",
		Value:         value,
		Version:       defaultVer,
		UserSubmitted: false,
	})
}

func (s *performanceStatistics) perfTotals() []model.PerfRollupValue {
	rollups := []model.PerfRollupValue{}

	return append(
		rollups,
		model.PerfRollupValue{
			Name:          "totalTime",
			Value:         s.timers.durationTotal,
			Version:       defaultVer,
			UserSubmitted: false,
		},
		model.PerfRollupValue{
			Name:          "totalFailures",
			Value:         s.gauges.failedTotal,
			Version:       defaultVer,
			UserSubmitted: false,
		},
		model.PerfRollupValue{
			Name:          "totalErrors",
			Value:         s.counters.errors,
			Version:       defaultVer,
			UserSubmitted: false,
		},
		model.PerfRollupValue{
			Name:          "totalOperations",
			Value:         s.counters.operations,
			Version:       defaultVer,
			UserSubmitted: false,
		},
		model.PerfRollupValue{
			Name:          "totalSize",
			Value:         s.counters.size,
			Version:       defaultVer,
			UserSubmitted: false,
		},
		model.PerfRollupValue{
			Name:          "totalSamples",
			Value:         s.numSamples,
			Version:       defaultVer,
			UserSubmitted: false,
		},
	)
}
