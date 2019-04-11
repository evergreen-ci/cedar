package perf

import (
	"github.com/aclements/go-moremath/stats"
	"github.com/evergreen-ci/cedar/model"
)

type RollupFactory interface {
	Names() []string
	Version() int
	Calc(*PerformanceStatistics, bool) []model.PerfRollupValue
}

var defaultRollups = []RollupFactory{
	&latencyAverage{},
	&sizeAverage{},
	&operationThroughput{},
	&sizeThroughput{},
	&errorThroughput{},
	&latencyPercentile{},
	&workersBounds{},
	&latencyBounds{},
	&durationSum{},
	&errorsSum{},
	&operationsSum{},
	&sizeSum{},
}

func DefaultRollupFactories() []RollupFactory { return defaultRollups }

//////////////////
// Default Means
//////////////////
type latencyAverage struct{}

const latencyAverageName = "AverageLatency"
const latencyAverageVersion = 3

func (f *latencyAverage) Names() []string { return []string{latencyAverageName} }
func (f *latencyAverage) Version() int    { return latencyAverageVersion }
func (f *latencyAverage) Calc(s *PerformanceStatistics, user bool) []model.PerfRollupValue {
	rollups := []model.PerfRollupValue{}
	if s.counters.operationsTotal > 0 {
		rollups = append(
			rollups,
			model.PerfRollupValue{
				Name:          latencyAverageName,
				Value:         float64(s.timers.durationTotal) / float64(s.counters.operationsTotal),
				Version:       latencyAverageVersion,
				MetricType:    model.MetricTypeMean,
				UserSubmitted: user,
			},
		)
	}

	return rollups
}

type sizeAverage struct{}

const sizeAverageName = "AverageSize"
const sizeAverageVersion = 3

func (f *sizeAverage) Names() []string { return []string{sizeAverageName} }
func (f *sizeAverage) Version() int    { return sizeAverageVersion }
func (f *sizeAverage) Calc(s *PerformanceStatistics, user bool) []model.PerfRollupValue {
	rollups := []model.PerfRollupValue{}
	if s.counters.operationsTotal > 0 {
		rollups = append(
			rollups,
			model.PerfRollupValue{
				Name:          sizeAverageName,
				Value:         float64(s.counters.sizeTotal) / float64(s.counters.operationsTotal),
				Version:       sizeAverageVersion,
				MetricType:    model.MetricTypeMean,
				UserSubmitted: user,
			},
		)
	}

	return rollups
}

////////////////////////
// Default Throughputs
////////////////////////
type operationThroughput struct{}

const operationThroughputName = "OperationThroughput"
const operationThroughputVersion = 3

func (f *operationThroughput) Names() []string { return []string{operationThroughputName} }
func (f *operationThroughput) Version() int    { return operationThroughputVersion }
func (f *operationThroughput) Calc(s *PerformanceStatistics, user bool) []model.PerfRollupValue {
	rollups := []model.PerfRollupValue{}
	if s.timers.durationTotal > 0 {
		rollups = append(
			rollups,
			model.PerfRollupValue{
				Name:          operationThroughputName,
				Value:         float64(s.counters.operationsTotal) / s.timers.durationTotal.Seconds(),
				Version:       operationThroughputVersion,
				MetricType:    model.MetricTypeThroughput,
				UserSubmitted: user,
			},
		)
	}

	return rollups
}

type sizeThroughput struct{}

const sizeThroughputName = "SizeThroughput"
const sizeThroughputVersion = 3

func (f *sizeThroughput) Names() []string { return []string{sizeThroughputName} }
func (f *sizeThroughput) Version() int    { return sizeThroughputVersion }
func (f *sizeThroughput) Calc(s *PerformanceStatistics, user bool) []model.PerfRollupValue {
	rollups := []model.PerfRollupValue{}
	if s.timers.durationTotal > 0 {
		rollups = append(
			rollups,
			model.PerfRollupValue{
				Name:          sizeThroughputName,
				Value:         float64(s.counters.sizeTotal) / s.timers.durationTotal.Seconds(),
				Version:       sizeThroughputVersion,
				MetricType:    model.MetricTypeThroughput,
				UserSubmitted: user,
			},
		)
	}

	return rollups
}

type errorThroughput struct{}

const errorThroughputName = "ErrorRate"
const errorThroughputVersion = 3

func (f *errorThroughput) Names() []string { return []string{errorThroughputName} }
func (f *errorThroughput) Version() int    { return errorThroughputVersion }
func (f *errorThroughput) Calc(s *PerformanceStatistics, user bool) []model.PerfRollupValue {
	rollups := []model.PerfRollupValue{}
	if s.timers.durationTotal > 0 {
		rollups = append(
			rollups,
			model.PerfRollupValue{
				Name:          errorThroughputName,
				Value:         float64(s.counters.errorsTotal) / s.timers.durationTotal.Seconds(),
				Version:       errorThroughputVersion,
				MetricType:    model.MetricTypeThroughput,
				UserSubmitted: user,
			},
		)
	}

	return rollups
}

////////////////////////
// Default Percentiles
////////////////////////
type latencyPercentile struct{}

const latencyPercentile50Name = "Latency50thPercentile"
const latencyPercentile80Name = "Latency80thPercentile"
const latencyPercentile90Name = "Latency90thPercentile"
const latencyPercentile95Name = "Latency95thPercentile"
const latencyPercentile99Name = "Latency99thPercentile"
const latencyPercentileVersion = 3

func (f *latencyPercentile) Names() []string {
	return []string{
		latencyPercentile50Name,
		latencyPercentile80Name,
		latencyPercentile90Name,
		latencyPercentile95Name,
		latencyPercentile99Name,
	}
}
func (f *latencyPercentile) Version() int { return latencyPercentileVersion }
func (f *latencyPercentile) Calc(s *PerformanceStatistics, user bool) []model.PerfRollupValue {
	rollups := []model.PerfRollupValue{}
	// probably should sort this for better efficiency
	latencySample := stats.Sample{Xs: s.timers.extractedDurations}
	if len(s.timers.extractedDurations) > 0 {
		rollups = append(
			rollups,
			model.PerfRollupValue{
				Name:          latencyPercentile50Name,
				Value:         latencySample.Quantile(0.5),
				Version:       latencyPercentileVersion,
				MetricType:    model.MetricTypePercentile50,
				UserSubmitted: user,
			},
			model.PerfRollupValue{
				Name:          latencyPercentile80Name,
				Value:         latencySample.Quantile(0.8),
				Version:       latencyPercentileVersion,
				MetricType:    model.MetricTypePercentile80,
				UserSubmitted: user,
			},
			model.PerfRollupValue{
				Name:          latencyPercentile90Name,
				Value:         latencySample.Quantile(0.9),
				Version:       latencyPercentileVersion,
				MetricType:    model.MetricTypePercentile90,
				UserSubmitted: user,
			},
			model.PerfRollupValue{
				Name:          latencyPercentile95Name,
				Value:         latencySample.Quantile(0.95),
				Version:       latencyPercentileVersion,
				MetricType:    model.MetricTypePercentile95,
				UserSubmitted: user,
			},
			model.PerfRollupValue{
				Name:          latencyPercentile99Name,
				Value:         latencySample.Quantile(0.99),
				Version:       latencyPercentileVersion,
				MetricType:    model.MetricTypePercentile99,
				UserSubmitted: user,
			},
		)
	}

	return rollups
}

///////////////////
// Default Bounds
///////////////////
type workersBounds struct{}

const workersMinName = "WorkersMin"
const workersMaxName = "WorkersMax"
const workersBoundsVersion = 3

func (f *workersBounds) Names() []string { return []string{workersMinName, workersMaxName} }
func (f *workersBounds) Version() int    { return workersBoundsVersion }
func (f *workersBounds) Calc(s *PerformanceStatistics, user bool) []model.PerfRollupValue {
	rollups := []model.PerfRollupValue{}
	if len(s.gauges.workers) > 0 {
		min, max := stats.Sample{Xs: s.gauges.workers}.Bounds()
		rollups = append(
			rollups,
			model.PerfRollupValue{
				Name:          workersMinName,
				Value:         min,
				Version:       workersBoundsVersion,
				MetricType:    model.MetricTypeMin,
				UserSubmitted: user,
			},
			model.PerfRollupValue{
				Name:          workersMaxName,
				Value:         max,
				Version:       workersBoundsVersion,
				MetricType:    model.MetricTypeMax,
				UserSubmitted: user,
			},
		)
	}

	return rollups
}

type latencyBounds struct{}

const latencyMinName = "LatencyMin"
const latencyMaxName = "LatencyMax"
const latencyBoundsVersion = 3

func (f *latencyBounds) Names() []string { return []string{latencyMinName, latencyMaxName} }
func (f *latencyBounds) Version() int    { return latencyBoundsVersion }
func (f *latencyBounds) Calc(s *PerformanceStatistics, user bool) []model.PerfRollupValue {
	rollups := []model.PerfRollupValue{}
	if len(s.timers.extractedDurations) > 0 {
		min, max := stats.Sample{Xs: s.timers.extractedDurations}.Bounds()
		rollups = append(
			rollups,
			model.PerfRollupValue{
				Name:          latencyMinName,
				Value:         min,
				Version:       latencyBoundsVersion,
				MetricType:    model.MetricTypeMin,
				UserSubmitted: user,
			},
			model.PerfRollupValue{
				Name:          latencyMaxName,
				Value:         max,
				Version:       latencyBoundsVersion,
				MetricType:    model.MetricTypeMax,
				UserSubmitted: user,
			},
		)
	}

	return rollups
}

/////////////////
// Default Sums
/////////////////
type durationSum struct{}

const durationSumName = "DurationTotal"
const durationSumVersion = 3

func (f *durationSum) Names() []string { return []string{durationSumName} }
func (f *durationSum) Version() int    { return durationSumVersion }
func (f *durationSum) Calc(s *PerformanceStatistics, user bool) []model.PerfRollupValue {
	return []model.PerfRollupValue{
		model.PerfRollupValue{
			Name:          durationSumName,
			Value:         s.timers.durationTotal,
			Version:       durationSumVersion,
			MetricType:    model.MetricTypeSum,
			UserSubmitted: user,
		},
	}
}

type errorsSum struct{}

const errorsSumName = "ErrorsTotal"
const errorsSumVersion = 3

func (f *errorsSum) Names() []string { return []string{errorsSumName} }
func (f *errorsSum) Version() int    { return errorsSumVersion }
func (f *errorsSum) Calc(s *PerformanceStatistics, user bool) []model.PerfRollupValue {
	return []model.PerfRollupValue{
		model.PerfRollupValue{
			Name:          errorsSumName,
			Value:         s.counters.errorsTotal,
			Version:       errorsSumVersion,
			MetricType:    model.MetricTypeSum,
			UserSubmitted: user,
		},
	}
}

type operationsSum struct{}

const operationsSumName = "OperationsTotal"
const operationsSumVersion = 3

func (f *operationsSum) Names() []string { return []string{operationsSumName} }
func (f *operationsSum) Version() int    { return operationsSumVersion }
func (f *operationsSum) Calc(s *PerformanceStatistics, user bool) []model.PerfRollupValue {
	return []model.PerfRollupValue{
		model.PerfRollupValue{
			Name:          operationsSumName,
			Value:         s.counters.operationsTotal,
			Version:       operationsSumVersion,
			MetricType:    model.MetricTypeSum,
			UserSubmitted: user,
		},
	}
}

type sizeSum struct{}

const sizeSumName = "SizeTotal"
const sizeSumVersion = 3

func (f *sizeSum) Names() []string { return []string{sizeSumName} }
func (f *sizeSum) Version() int    { return sizeSumVersion }
func (f *sizeSum) Calc(s *PerformanceStatistics, user bool) []model.PerfRollupValue {
	return []model.PerfRollupValue{
		model.PerfRollupValue{
			Name:          sizeSumName,
			Value:         s.counters.sizeTotal,
			Version:       sizeSumVersion,
			MetricType:    model.MetricTypeSum,
			UserSubmitted: user,
		},
	}
}
