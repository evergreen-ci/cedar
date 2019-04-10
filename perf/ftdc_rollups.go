package perf

import (
	"time"

	"github.com/aclements/go-moremath/stats"
	"github.com/evergreen-ci/cedar/model"
	"github.com/mongodb/ftdc"
	"github.com/pkg/errors"
)

type performanceStatistics struct {
	counters struct {
		operationsTotal int64
		sizeTotal       int64
		errorsTotal     int64
	}

	timers struct {
		extractedDurations []float64
		durationTotal      time.Duration
		total              time.Duration
	}

	gauges struct {
		state   []float64
		workers []float64
		failed  []float64
	}
}

func CalculateDefaultRollups(dx *ftdc.ChunkIterator) ([]model.PerfRollupValue, error) {
	rollups := []model.PerfRollupValue{}

	perfStats, err := createPerformanceStats(dx)
	if err != nil {
		return rollups, errors.Wrap(err, "problem calculating perf rollups")
	}

	rollups = append(rollups, perfStats.means()...)
	rollups = append(rollups, perfStats.throughputs()...)
	rollups = append(rollups, perfStats.percentiles()...)
	rollups = append(rollups, perfStats.bounds()...)
	rollups = append(rollups, perfStats.sums()...)
	return rollups, nil
}

func createPerformanceStats(dx *ftdc.ChunkIterator) (performanceStatistics, error) {
	perfStats := performanceStatistics{}

	defer dx.Close()
	for i := 0; dx.Next(); i++ {
		chunk := dx.Chunk()

		for _, metric := range chunk.Metrics {
			switch name := metric.Key(); name {
			case "counters.ops":
				perfStats.counters.operationsTotal = metric.Values[len(metric.Values)-1]
			case "counters.size":
				perfStats.counters.sizeTotal = metric.Values[len(metric.Values)-1]
			case "counters.errors":
				perfStats.counters.errorsTotal = metric.Values[len(metric.Values)-1]
			case "timers.duration", "timers.dur":
				perfStats.timers.extractedDurations = extractValues(convertToFloats(metric.Values))
				perfStats.timers.durationTotal = time.Duration(metric.Values[len(metric.Values)-1])
			case "timers.total":
				perfStats.timers.total = time.Duration(metric.Values[len(metric.Values)-1])
			case "gauges.state":
				perfStats.gauges.state = convertToFloats(metric.Values)
			case "gauges.workers":
				perfStats.gauges.workers = convertToFloats(metric.Values)
			case "gauges.failed":
				perfStats.gauges.failed = convertToFloats(metric.Values)
			case "ts", "counters.n", "id":
				continue
			default:
				return performanceStatistics{}, errors.Errorf("unknown field name %s", name)
			}
		}
	}
	return perfStats, errors.WithStack(dx.Err())
}

func (s *performanceStatistics) means() []model.PerfRollupValue {
	rollups := []model.PerfRollupValue{}

	if s.counters.operationsTotal > 0 {
		if s.timers.durationTotal > 0 {
			rollups = append(
				rollups,
				model.PerfRollupValue{
					Name:       "AverageLatency",
					Value:      float64(s.timers.durationTotal) / float64(s.counters.operationsTotal),
					Version:    model.DefaultVer,
					MetricType: model.MetricTypeMean,
				},
			)
		}
		rollups = append(
			rollups,
			model.PerfRollupValue{
				Name:       "AverageOperationSize",
				Value:      float64(s.counters.sizeTotal) / float64(s.counters.operationsTotal),
				Version:    model.DefaultVer,
				MetricType: model.MetricTypeMean,
			},
		)
	}
	return rollups
}

func (s *performanceStatistics) throughputs() []model.PerfRollupValue {
	rollups := []model.PerfRollupValue{}

	if s.timers.durationTotal > 0 {
		rollups = append(
			rollups,
			model.PerfRollupValue{
				Name:       "OperationThroughput",
				Value:      float64(s.counters.operationsTotal) / s.timers.durationTotal.Seconds(),
				Version:    model.DefaultVer,
				MetricType: model.MetricTypeThroughput,
			},
			model.PerfRollupValue{
				Name:       "SizeThroughput",
				Value:      float64(s.counters.sizeTotal) / s.timers.durationTotal.Seconds(),
				Version:    model.DefaultVer,
				MetricType: model.MetricTypeThroughput,
			},
			model.PerfRollupValue{
				Name:       "ErrorRate",
				Value:      float64(s.counters.errorsTotal) / s.timers.durationTotal.Seconds(),
				Version:    model.DefaultVer,
				MetricType: model.MetricTypeThroughput,
			},
		)
	}
	return rollups
}

func (s *performanceStatistics) percentiles() []model.PerfRollupValue {
	rollups := []model.PerfRollupValue{}

	latencySample := stats.Sample{Xs: s.timers.extractedDurations}
	if len(s.timers.extractedDurations) > 0 {
		rollups = append(
			rollups,
			model.PerfRollupValue{
				Name:       "Latency50thPercentile",
				Value:      latencySample.Quantile(0.5),
				MetricType: model.MetricTypePercentile50,
			},
			model.PerfRollupValue{
				Name:       "Latency80thPercentile",
				Value:      latencySample.Quantile(0.8),
				MetricType: model.MetricTypePercentile80,
			},
			model.PerfRollupValue{
				Name:       "Latency90thPercentile",
				Value:      latencySample.Quantile(0.9),
				MetricType: model.MetricTypePercentile90,
			},
			model.PerfRollupValue{
				Name:       "Latency95thPercentile",
				Value:      latencySample.Quantile(0.95),
				MetricType: model.MetricTypePercentile95,
			},
			model.PerfRollupValue{
				Name:       "Latency99thPercentile",
				Value:      latencySample.Quantile(0.99),
				MetricType: model.MetricTypePercentile99,
			},
		)
	}
	return rollups
}

func (s *performanceStatistics) bounds() []model.PerfRollupValue {
	rollups := []model.PerfRollupValue{}

	min, max := stats.Sample{Xs: s.gauges.workers}.Bounds()
	rollups = append(
		rollups,
		model.PerfRollupValue{
			Name:       "WorkersMin",
			Value:      min,
			MetricType: model.MetricTypeMin,
		},
		model.PerfRollupValue{
			Name:       "WorkersMax",
			Value:      max,
			MetricType: model.MetricTypeMax,
		},
	)
	min, max = stats.Sample{Xs: s.timers.extractedDurations}.Bounds()
	rollups = append(
		rollups,
		model.PerfRollupValue{
			Name:       "LatencyMin",
			Value:      min,
			MetricType: model.MetricTypeMin,
		},
		model.PerfRollupValue{
			Name:       "LatencyMax",
			Value:      max,
			MetricType: model.MetricTypeMax,
		},
	)
	return rollups
}

func (s *performanceStatistics) sums() []model.PerfRollupValue {
	rollups := []model.PerfRollupValue{}

	return append(
		rollups,
		model.PerfRollupValue{
			Name:       "DurationTotal",
			Value:      s.timers.durationTotal,
			Version:    model.DefaultVer,
			MetricType: model.MetricTypeSum,
		},
		model.PerfRollupValue{
			Name:       "ErrorsTotal",
			Value:      s.counters.errorsTotal,
			Version:    model.DefaultVer,
			MetricType: model.MetricTypeSum,
		},
		model.PerfRollupValue{
			Name:       "OperationsTotal",
			Value:      s.counters.operationsTotal,
			Version:    model.DefaultVer,
			MetricType: model.MetricTypeSum,
		},
		model.PerfRollupValue{
			Name:       "SizeTotal",
			Value:      s.counters.sizeTotal,
			Version:    model.DefaultVer,
			MetricType: model.MetricTypeSum,
		},
	)
}

func convertToFloats(ints []int64) []float64 {
	floats := []float64{}
	for i := range ints {
		floats = append(floats, float64(ints[i]))
	}

	return floats
}

// expects slice of cumulative values
func extractValues(vals []float64) []float64 {
	extractedVals := make([]float64, len(vals))

	lastValue := float64(0)
	for i := range vals {
		extractedVals[i] = vals[i] - lastValue
		lastValue = vals[i]
	}

	return extractedVals
}
