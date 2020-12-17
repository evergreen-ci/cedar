package perf

import (
	"sort"

	"github.com/aclements/go-moremath/stats"
	"github.com/evergreen-ci/cedar/model"
)

type RollupFactory interface {
	Type() string
	Names() []string
	Version() int
	Calc(*PerformanceStatistics, bool) []model.PerfRollupValue
}

var rollupsMap = map[string]RollupFactory{
	latencyAverageName:      &latencyAverage{},
	sizeAverageName:         &sizeAverage{},
	operationThroughputName: &operationThroughput{},
	sizeThroughputName:      &sizeThroughput{},
	errorThroughputName:     &errorThroughput{},
	latencyPercentileName:   &latencyPercentile{},
	workersBoundsName:       &workersBounds{},
	latencyBoundsName:       &latencyBounds{},
	durationSumName:         &durationSum{},
	errorsSumName:           &errorsSum{},
	operationsSumName:       &operationsSum{},
	sizeSumName:             &sizeSum{},
	overheadSumName:         &overheadSum{},
}

func RollupsMap() map[string]RollupFactory {
	return rollupsMap
}

func RollupFactoryFromType(t string) RollupFactory {
	return rollupsMap[t]
}

var defaultRollups = []RollupFactory{
	&latencyAverage{},
	&sizeAverage{},
	&operationThroughput{},
	&documentThroughput{},
	&sizeThroughput{},
	&errorThroughput{},
	&latencyPercentile{},
	&workersBounds{},
	&latencyBounds{},
	&durationSum{},
	&errorsSum{},
	&operationsSum{},
	&documentsSum{},
	&sizeSum{},
	&overheadSum{},
}

func DefaultRollupFactories() []RollupFactory { return defaultRollups }

//////////////////
// Default Means
//////////////////
type latencyAverage struct{}

const (
	latencyAverageName    = "AverageLatency"
	latencyAverageVersion = 3
)

func (f *latencyAverage) Type() string    { return latencyAverageName }
func (f *latencyAverage) Names() []string { return []string{latencyAverageName} }
func (f *latencyAverage) Version() int    { return latencyAverageVersion }
func (f *latencyAverage) Calc(s *PerformanceStatistics, user bool) []model.PerfRollupValue {
	rollup := model.PerfRollupValue{
		Name:          latencyAverageName,
		Version:       latencyAverageVersion,
		MetricType:    model.MetricTypeMean,
		UserSubmitted: user,
	}

	if s.counters.operationsTotal > 0 {
		rollup.Value = float64(s.timers.durationTotal) / float64(s.counters.operationsTotal)
	}

	return []model.PerfRollupValue{rollup}
}

type sizeAverage struct{}

const (
	sizeAverageName    = "AverageSize"
	sizeAverageVersion = 3
)

func (f *sizeAverage) Type() string    { return sizeAverageName }
func (f *sizeAverage) Names() []string { return []string{sizeAverageName} }
func (f *sizeAverage) Version() int    { return sizeAverageVersion }
func (f *sizeAverage) Calc(s *PerformanceStatistics, user bool) []model.PerfRollupValue {
	rollup := model.PerfRollupValue{
		Name:          sizeAverageName,
		Version:       sizeAverageVersion,
		MetricType:    model.MetricTypeMean,
		UserSubmitted: user,
	}

	if s.counters.operationsTotal > 0 {
		rollup.Value = float64(s.counters.sizeTotal) / float64(s.counters.operationsTotal)
	}

	return []model.PerfRollupValue{rollup}
}

////////////////////////
// Default Throughputs
////////////////////////
type operationThroughput struct{}

const (
	operationThroughputName    = "OperationThroughput"
	operationThroughputVersion = 4
)

func (f *operationThroughput) Type() string    { return operationThroughputName }
func (f *operationThroughput) Names() []string { return []string{operationThroughputName} }
func (f *operationThroughput) Version() int    { return operationThroughputVersion }
func (f *operationThroughput) Calc(s *PerformanceStatistics, user bool) []model.PerfRollupValue {
	rollup := model.PerfRollupValue{
		Name:          operationThroughputName,
		Version:       operationThroughputVersion,
		MetricType:    model.MetricTypeThroughput,
		UserSubmitted: user,
	}

	if s.timers.totalWallTime > 0 {
		rollup.Value = float64(s.counters.operationsTotal) / s.timers.totalWallTime.Seconds()
	}

	return []model.PerfRollupValue{rollup}
}

type documentThroughput struct{}

const (
	documentThroughputName    = "DocumentThroughput"
	documentThroughputVersion = 0
)

func (f *documentThroughput) Type() string    { return documentThroughputName }
func (f *documentThroughput) Names() []string { return []string{documentThroughputName} }
func (f *documentThroughput) Version() int    { return documentThroughputVersion }
func (f *documentThroughput) Calc(s *PerformanceStatistics, user bool) []model.PerfRollupValue {
	rollup := model.PerfRollupValue{
		Name:          documentThroughputName,
		Version:       documentThroughputVersion,
		MetricType:    model.MetricTypeThroughput,
		UserSubmitted: user,
	}

	if s.timers.totalWallTime > 0 {
		rollup.Value = float64(s.counters.documentsTotal) / s.timers.totalWallTime.Seconds()
	}

	return []model.PerfRollupValue{rollup}
}

type sizeThroughput struct{}

const (
	sizeThroughputName    = "SizeThroughput"
	sizeThroughputVersion = 4
)

func (f *sizeThroughput) Type() string    { return sizeThroughputName }
func (f *sizeThroughput) Names() []string { return []string{sizeThroughputName} }
func (f *sizeThroughput) Version() int    { return sizeThroughputVersion }
func (f *sizeThroughput) Calc(s *PerformanceStatistics, user bool) []model.PerfRollupValue {
	rollup := model.PerfRollupValue{
		Name:          sizeThroughputName,
		Version:       sizeThroughputVersion,
		MetricType:    model.MetricTypeThroughput,
		UserSubmitted: user,
	}
	if s.timers.totalWallTime > 0 {
		rollup.Value = float64(s.counters.sizeTotal) / s.timers.totalWallTime.Seconds()
	}

	return []model.PerfRollupValue{rollup}
}

type errorThroughput struct{}

const (
	errorThroughputName    = "ErrorRate"
	errorThroughputVersion = 4
)

func (f *errorThroughput) Type() string    { return errorThroughputName }
func (f *errorThroughput) Names() []string { return []string{errorThroughputName} }
func (f *errorThroughput) Version() int    { return errorThroughputVersion }
func (f *errorThroughput) Calc(s *PerformanceStatistics, user bool) []model.PerfRollupValue {
	rollup := model.PerfRollupValue{
		Name:          errorThroughputName,
		Version:       errorThroughputVersion,
		MetricType:    model.MetricTypeThroughput,
		UserSubmitted: user,
	}

	if s.timers.totalWallTime > 0 {
		rollup.Value = float64(s.counters.errorsTotal) / s.timers.totalWallTime.Seconds()
	}

	return []model.PerfRollupValue{rollup}
}

////////////////////////
// Default Percentiles
////////////////////////
type latencyPercentile struct{}

const (
	latencyPercentileName    = "LatencyPercentile"
	latencyPercentile50Name  = "Latency50thPercentile"
	latencyPercentile80Name  = "Latency80thPercentile"
	latencyPercentile90Name  = "Latency90thPercentile"
	latencyPercentile95Name  = "Latency95thPercentile"
	latencyPercentile99Name  = "Latency99thPercentile"
	latencyPercentileVersion = 4
)

func (f *latencyPercentile) Type() string { return latencyPercentileName }
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
	p50 := model.PerfRollupValue{
		Name:          latencyPercentile50Name,
		Version:       latencyPercentileVersion,
		MetricType:    model.MetricTypePercentile50,
		UserSubmitted: user,
	}
	p80 := model.PerfRollupValue{
		Name:          latencyPercentile80Name,
		Version:       latencyPercentileVersion,
		MetricType:    model.MetricTypePercentile80,
		UserSubmitted: user,
	}
	p90 := model.PerfRollupValue{
		Name:          latencyPercentile90Name,
		Version:       latencyPercentileVersion,
		MetricType:    model.MetricTypePercentile90,
		UserSubmitted: user,
	}
	p95 := model.PerfRollupValue{
		Name:          latencyPercentile95Name,
		Version:       latencyPercentileVersion,
		MetricType:    model.MetricTypePercentile95,
		UserSubmitted: user,
	}
	p99 := model.PerfRollupValue{
		Name:          latencyPercentile99Name,
		Version:       latencyPercentileVersion,
		MetricType:    model.MetricTypePercentile99,
		UserSubmitted: user,
	}

	if len(s.timers.extractedDurations) > 0 {
		durs := make(sort.Float64Slice, len(s.timers.extractedDurations))
		copy(durs, s.timers.extractedDurations)
		durs.Sort()
		latencySample := stats.Sample{
			Xs:     durs,
			Sorted: true,
		}
		p50.Value = latencySample.Quantile(0.5)
		p80.Value = latencySample.Quantile(0.8)
		p90.Value = latencySample.Quantile(0.9)
		p95.Value = latencySample.Quantile(0.95)
		p99.Value = latencySample.Quantile(0.99)
	}

	return []model.PerfRollupValue{p50, p80, p90, p95, p99}
}

///////////////////
// Default Bounds
///////////////////
type workersBounds struct{}

const (
	workersBoundsName    = "WorkersBounds"
	workersMinName       = "WorkersMin"
	workersMaxName       = "WorkersMax"
	workersBoundsVersion = 3
)

func (f *workersBounds) Type() string    { return workersBoundsName }
func (f *workersBounds) Names() []string { return []string{workersMinName, workersMaxName} }
func (f *workersBounds) Version() int    { return workersBoundsVersion }
func (f *workersBounds) Calc(s *PerformanceStatistics, user bool) []model.PerfRollupValue {
	min := model.PerfRollupValue{
		Name:          workersMinName,
		Version:       workersBoundsVersion,
		MetricType:    model.MetricTypeMin,
		UserSubmitted: user,
	}
	max := model.PerfRollupValue{
		Name:          workersMaxName,
		Version:       workersBoundsVersion,
		MetricType:    model.MetricTypeMax,
		UserSubmitted: user,
	}
	if len(s.gauges.workers) > 0 {
		min.Value, max.Value = stats.Sample{Xs: s.gauges.workers}.Bounds()
	}

	return []model.PerfRollupValue{min, max}
}

type latencyBounds struct{}

const (
	latencyBoundsName    = "LatencyBounds"
	latencyMinName       = "LatencyMin"
	latencyMaxName       = "LatencyMax"
	latencyBoundsVersion = 4
)

func (f *latencyBounds) Type() string    { return latencyBoundsName }
func (f *latencyBounds) Names() []string { return []string{latencyMinName, latencyMaxName} }
func (f *latencyBounds) Version() int    { return latencyBoundsVersion }
func (f *latencyBounds) Calc(s *PerformanceStatistics, user bool) []model.PerfRollupValue {
	min := model.PerfRollupValue{
		Name:          latencyMinName,
		Version:       latencyBoundsVersion,
		MetricType:    model.MetricTypeMin,
		UserSubmitted: user,
	}
	max := model.PerfRollupValue{
		Name:          latencyMaxName,
		Version:       latencyBoundsVersion,
		MetricType:    model.MetricTypeMax,
		UserSubmitted: user,
	}

	if len(s.timers.extractedDurations) > 0 {
		min.Value, max.Value = stats.Sample{Xs: s.timers.extractedDurations}.Bounds()
	}

	return []model.PerfRollupValue{min, max}
}

/////////////////
// Default Sums
/////////////////
type durationSum struct{}

const (
	durationSumName    = "DurationTotal"
	durationSumVersion = 4
)

func (f *durationSum) Type() string    { return durationSumName }
func (f *durationSum) Names() []string { return []string{durationSumName} }
func (f *durationSum) Version() int    { return durationSumVersion }
func (f *durationSum) Calc(s *PerformanceStatistics, user bool) []model.PerfRollupValue {
	return []model.PerfRollupValue{
		model.PerfRollupValue{
			Name:          durationSumName,
			Value:         s.timers.totalWallTime,
			Version:       durationSumVersion,
			MetricType:    model.MetricTypeSum,
			UserSubmitted: user,
		},
	}
}

type errorsSum struct{}

const (
	errorsSumName    = "ErrorsTotal"
	errorsSumVersion = 3
)

func (f *errorsSum) Type() string    { return errorsSumName }
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

const (
	operationsSumName    = "OperationsTotal"
	operationsSumVersion = 3
)

func (f *operationsSum) Type() string    { return operationsSumName }
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

type documentsSum struct{}

const (
	documentsSumName    = "DocumentsTotal"
	documentsSumVersion = 0
)

func (f *documentsSum) Type() string    { return documentsSumName }
func (f *documentsSum) Names() []string { return []string{documentsSumName} }
func (f *documentsSum) Version() int    { return documentsSumVersion }
func (f *documentsSum) Calc(s *PerformanceStatistics, user bool) []model.PerfRollupValue {
	return []model.PerfRollupValue{
		model.PerfRollupValue{
			Name:          documentsSumName,
			Value:         s.counters.documentsTotal,
			Version:       documentsSumVersion,
			MetricType:    model.MetricTypeSum,
			UserSubmitted: user,
		},
	}
}

type sizeSum struct{}

const (
	sizeSumName    = "SizeTotal"
	sizeSumVersion = 3
)

func (f *sizeSum) Type() string    { return sizeSumName }
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

type overheadSum struct{}

const (
	overheadSumName    = "OverheadTotal"
	overheadSumVersion = 1
)

func (f *overheadSum) Type() string    { return overheadSumName }
func (f *overheadSum) Names() []string { return []string{overheadSumName} }
func (f *overheadSum) Version() int    { return overheadSumVersion }
func (f *overheadSum) Calc(s *PerformanceStatistics, user bool) []model.PerfRollupValue {
	return []model.PerfRollupValue{
		model.PerfRollupValue{
			Name:          overheadSumName,
			Value:         s.timers.total - s.timers.durationTotal,
			Version:       overheadSumVersion,
			MetricType:    model.MetricTypeSum,
			UserSubmitted: user,
		},
	}
}
