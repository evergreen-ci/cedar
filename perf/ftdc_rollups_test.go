package perf

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/aclements/go-moremath/stats"
	"github.com/evergreen-ci/cedar/model"
	"github.com/mongodb/ftdc"
	"github.com/mongodb/ftdc/bsonx"
	"github.com/mongodb/ftdc/events"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateDefaultRollups(t *testing.T) {
	for _, test := range []struct {
		name    string
		samples int
		err     bool
	}{
		{
			name:    "TestExpectedInput",
			samples: 100,
		},
		{
			name:    "TestInvalidInput",
			samples: 10,
			err:     true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			data, err := createFTDC(!test.err, test.samples)
			require.NoError(t, err)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ci := ftdc.ReadChunks(ctx, bytes.NewReader(data))

			actual, err := CalculateDefaultRollups(ci, false)
			if test.err {
				assert.Equal(t, []model.PerfRollupValue{}, actual)
				assert.Error(t, err)
			} else {
				assert.Equal(t, 21, len(actual))
				assert.NoError(t, err)
			}
		})
	}
}

func TestCalcFunctions(t *testing.T) {
	s := &PerformanceStatistics{
		counters: struct {
			operationsTotal int64
			documentsTotal  int64
			sizeTotal       int64
			errorsTotal     int64
		}{
			operationsTotal: 10,
			documentsTotal:  5,
			sizeTotal:       1000,
			errorsTotal:     5,
		},
		timers: struct {
			extractedDurations []float64
			durationTotal      time.Duration
			total              time.Duration
			totalWallTime      time.Duration
		}{
			extractedDurations: extractValues(convertToFloats([]int64{
				500000,
				600000,
				1200000,
				1900000,
				2400000,
				3000000,
				4000000,
				4400000,
				4800000,
				5000000,
			}), 0),
			durationTotal: time.Duration(500000000),
			total:         time.Duration(600000000),
			totalWallTime: 2 * time.Second,
		},
		gauges: struct {
			state   []float64
			workers []float64
			failed  []float64
		}{
			state:   convertToFloats([]int64{1, 5, 100, 5, 60, 90, 40, 30, 5, 1}),
			workers: convertToFloats([]int64{1, 100, 100, 100, 100, 100, 100, 100, 100, 2}),
			failed:  []float64{},
		},
	}
	expectedExtractedDurations := []float64{500000, 100000, 600000, 700000, 500000, 600000, 1000000, 400000, 400000, 200000}

	for _, test := range []struct {
		name                string
		factory             RollupFactory
		expectedValues      []interface{}
		expectedVersion     int
		expectedMetricTypes []model.MetricType
		zeroValue           func(RollupFactory, bool) []model.PerfRollupValue
	}{
		{
			name:    latencyAverageName,
			factory: &latencyAverage{},
			expectedValues: []interface{}{
				float64(s.timers.durationTotal) / float64(s.counters.operationsTotal),
			},
			expectedVersion:     latencyAverageVersion,
			expectedMetricTypes: []model.MetricType{model.MetricTypeMean},
			zeroValue: func(f RollupFactory, u bool) []model.PerfRollupValue {
				original := s.counters.operationsTotal
				s.counters.operationsTotal = 0
				rollups := f.Calc(s, u)
				s.counters.operationsTotal = original
				return rollups
			},
		},
		{
			name:    sizeAverageName,
			factory: &sizeAverage{},
			expectedValues: []interface{}{
				float64(s.counters.sizeTotal) / float64(s.counters.operationsTotal),
			},
			expectedVersion:     sizeAverageVersion,
			expectedMetricTypes: []model.MetricType{model.MetricTypeMean},
			zeroValue: func(f RollupFactory, u bool) []model.PerfRollupValue {
				original := s.counters.operationsTotal
				s.counters.operationsTotal = 0
				rollups := f.Calc(s, u)
				s.counters.operationsTotal = original
				return rollups
			},
		},
		{
			name:    operationThroughputName,
			factory: &operationThroughput{},
			expectedValues: []interface{}{
				float64(s.counters.operationsTotal) / s.timers.totalWallTime.Seconds(),
			},
			expectedVersion:     operationThroughputVersion,
			expectedMetricTypes: []model.MetricType{model.MetricTypeThroughput},
			zeroValue: func(f RollupFactory, u bool) []model.PerfRollupValue {
				original := s.timers.totalWallTime
				s.timers.totalWallTime = 0
				rollups := f.Calc(s, u)
				s.timers.totalWallTime = original
				return rollups
			},
		},
		{
			name:    documentThroughputName,
			factory: &documentThroughput{},
			expectedValues: []interface{}{
				float64(s.counters.documentsTotal) / s.timers.totalWallTime.Seconds(),
			},
			expectedVersion:     documentThroughputVersion,
			expectedMetricTypes: []model.MetricType{model.MetricTypeThroughput},
			zeroValue: func(f RollupFactory, u bool) []model.PerfRollupValue {
				original := s.timers.totalWallTime
				s.timers.totalWallTime = 0
				rollups := f.Calc(s, u)
				s.timers.totalWallTime = original
				return rollups
			},
		},
		{
			name:    sizeThroughputName,
			factory: &sizeThroughput{},
			expectedValues: []interface{}{
				float64(s.counters.sizeTotal) / s.timers.totalWallTime.Seconds(),
			},
			expectedVersion:     sizeThroughputVersion,
			expectedMetricTypes: []model.MetricType{model.MetricTypeThroughput},
			zeroValue: func(f RollupFactory, u bool) []model.PerfRollupValue {
				original := s.timers.totalWallTime
				s.timers.totalWallTime = 0
				rollups := f.Calc(s, u)
				s.timers.totalWallTime = original
				return rollups
			},
		},
		{
			name:    errorThroughputName,
			factory: &errorThroughput{},
			expectedValues: []interface{}{
				float64(s.counters.errorsTotal) / s.timers.totalWallTime.Seconds(),
			},
			expectedVersion:     errorThroughputVersion,
			expectedMetricTypes: []model.MetricType{model.MetricTypeThroughput},
			zeroValue: func(f RollupFactory, u bool) []model.PerfRollupValue {
				original := s.timers.totalWallTime
				s.timers.totalWallTime = 0
				rollups := f.Calc(s, u)
				s.timers.totalWallTime = original
				return rollups
			},
		},
		{
			name:    latencyPercentileName,
			factory: &latencyPercentile{},
			expectedValues: []interface{}{
				stats.Sample{Xs: expectedExtractedDurations}.Quantile(0.5),
				stats.Sample{Xs: expectedExtractedDurations}.Quantile(0.8),
				stats.Sample{Xs: expectedExtractedDurations}.Quantile(0.9),
				stats.Sample{Xs: expectedExtractedDurations}.Quantile(0.95),
				stats.Sample{Xs: expectedExtractedDurations}.Quantile(0.99),
			},
			expectedVersion: latencyPercentileVersion,
			expectedMetricTypes: []model.MetricType{
				model.MetricTypePercentile50,
				model.MetricTypePercentile80,
				model.MetricTypePercentile90,
				model.MetricTypePercentile95,
				model.MetricTypePercentile99,
			},
			zeroValue: func(f RollupFactory, u bool) []model.PerfRollupValue {
				original := s.timers.extractedDurations
				s.timers.extractedDurations = nil
				rollups := f.Calc(s, u)
				s.timers.extractedDurations = original
				return rollups
			},
		},
		{
			name:                workersBoundsName,
			factory:             &workersBounds{},
			expectedValues:      []interface{}{float64(1), float64(100)},
			expectedVersion:     workersBoundsVersion,
			expectedMetricTypes: []model.MetricType{model.MetricTypeMin, model.MetricTypeMax},
			zeroValue: func(f RollupFactory, u bool) []model.PerfRollupValue {
				original := s.gauges.workers
				s.gauges.workers = nil
				rollups := f.Calc(s, u)
				s.gauges.workers = original
				return rollups
			},
		},
		{
			name:                latencyBoundsName,
			factory:             &latencyBounds{},
			expectedValues:      []interface{}{float64(100000), float64(1000000)},
			expectedVersion:     workersBoundsVersion,
			expectedMetricTypes: []model.MetricType{model.MetricTypeMin, model.MetricTypeMax},
			zeroValue: func(f RollupFactory, u bool) []model.PerfRollupValue {
				original := s.timers.extractedDurations
				s.timers.extractedDurations = nil
				rollups := f.Calc(s, u)
				s.timers.extractedDurations = original
				return rollups
			},
		},
		{
			name:                durationSumName,
			factory:             &durationSum{},
			expectedValues:      []interface{}{s.timers.totalWallTime},
			expectedVersion:     durationSumVersion,
			expectedMetricTypes: []model.MetricType{model.MetricTypeSum},
		},
		{
			name:                errorsSumName,
			factory:             &errorsSum{},
			expectedValues:      []interface{}{s.counters.errorsTotal},
			expectedVersion:     errorsSumVersion,
			expectedMetricTypes: []model.MetricType{model.MetricTypeSum},
		},
		{
			name:                operationsSumName,
			factory:             &operationsSum{},
			expectedValues:      []interface{}{s.counters.operationsTotal},
			expectedVersion:     operationsSumVersion,
			expectedMetricTypes: []model.MetricType{model.MetricTypeSum},
		},
		{
			name:                documentsSumName,
			factory:             &documentsSum{},
			expectedValues:      []interface{}{s.counters.documentsTotal},
			expectedVersion:     documentsSumVersion,
			expectedMetricTypes: []model.MetricType{model.MetricTypeSum},
		},
		{
			name:                sizeSumName,
			factory:             &sizeSum{},
			expectedValues:      []interface{}{s.counters.sizeTotal},
			expectedVersion:     sizeSumVersion,
			expectedMetricTypes: []model.MetricType{model.MetricTypeSum},
		},
		{
			name:                overheadSumName,
			factory:             &overheadSum{},
			expectedValues:      []interface{}{s.timers.total - s.timers.durationTotal},
			expectedVersion:     overheadSumVersion,
			expectedMetricTypes: []model.MetricType{model.MetricTypeSum},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, user := range []bool{true, false} {
				t.Run(fmt.Sprintf("UserSubmitted-%t", user), func(t *testing.T) {
					t.Run("Data", func(t *testing.T) {
						actual := test.factory.Calc(s, user)
						require.Equal(t, len(test.expectedValues), len(actual))
						for i, rollup := range actual {
							assert.Equal(t, test.factory.Names()[i], rollup.Name)
							assert.Equal(t, test.expectedValues[i], rollup.Value)
							assert.Equal(t, test.expectedMetricTypes[i], rollup.MetricType)
							assert.Equal(t, user, rollup.UserSubmitted)
						}
					})

					if test.zeroValue != nil {
						t.Run("ZeroValue", func(t *testing.T) {
							actual := test.zeroValue(test.factory, user)
							for i, rollup := range actual {
								assert.Equal(t, test.factory.Names()[i], rollup.Name)
								assert.Nil(t, rollup.Value)
								assert.Equal(t, test.expectedMetricTypes[i], rollup.MetricType)
								assert.Equal(t, user, rollup.UserSubmitted)
							}
						})
					}
				})
			}

		})
	}
}

func createFTDC(valid bool, samples int) ([]byte, error) {
	collector := ftdc.NewDynamicCollector(5)
	if valid {
		recorder := events.NewRawRecorder(collector)

		for i := 0; i < samples; i++ {
			recorder.Begin()
			recorder.IncOps(rand.Int63n(100) + 1)
			recorder.End(time.Duration(1000000))
		}
		if err := recorder.Flush(); err != nil {
			return []byte{}, errors.WithStack(err)
		}

	} else {
		for i := 0; i < samples; i++ {
			err := collector.Add(bsonx.NewDocument(bsonx.EC.Int64("one", rand.Int63()), bsonx.EC.Int64("two", rand.Int63())))
			if err != nil {
				return []byte{}, errors.WithStack(err)
			}
		}
	}

	data, err := collector.Resolve()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}
