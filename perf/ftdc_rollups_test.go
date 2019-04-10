package perf

import (
	"bytes"
	"context"
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

			actual, err := CalculateDefaultRollups(ci)
			if test.err {
				assert.Equal(t, []model.PerfRollupValue{}, actual)
				assert.Error(t, err)
			} else {
				assert.Equal(t, 18, len(actual))
				assert.NoError(t, err)
			}
		})
	}
}

func TestCalcFunctions(t *testing.T) {
	s := &performanceStatistics{
		counters: struct {
			operationsTotal int64
			sizeTotal       int64
			errorsTotal     int64
		}{
			operationsTotal: 10,
			sizeTotal:       1000,
			errorsTotal:     5,
		},
		timers: struct {
			extractedDurations []float64
			durationTotal      time.Duration
			total              time.Duration
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
			})),
			durationTotal: time.Duration(5000000),
			total:         time.Duration(6000000),
		},
		gauges: struct {
			state   []float64
			workers []float64
			failed  []float64
		}{
			state:   convertToFloats([]int64{1, 5, 100, 5, 60, 90, 40, 30, 5, 1}),
			workers: convertToFloats([]int64{1, 100, 100, 100, 100, 100, 100, 100, 100, 2}),
		},
	}
	expectedExtractedDurations := []float64{500000, 100000, 600000, 700000, 500000, 600000, 1000000, 400000, 400000, 200000}

	for _, test := range []struct {
		name               string
		function           func(*performanceStatistics) []model.PerfRollupValue
		expectedValue      []interface{}
		expectedMetricType []model.MetricType
		zeroValue          func()
	}{
		{
			name:     "TestMeans",
			function: (*performanceStatistics).means,
			expectedValue: []interface{}{
				float64(s.timers.durationTotal) / float64(s.counters.operationsTotal),
				float64(s.counters.sizeTotal) / float64(s.counters.operationsTotal),
			},
			expectedMetricType: []model.MetricType{
				model.MetricTypeMean,
				model.MetricTypeMean,
			},
			zeroValue: func() {
				original := s.counters.operationsTotal
				s.counters.operationsTotal = 0
				rollups := s.means()
				s.counters.operationsTotal = original
				assert.Equal(t, rollups, []model.PerfRollupValue{})
			},
		},
		{
			name:     "TestThroughputs",
			function: (*performanceStatistics).throughputs,
			expectedValue: []interface{}{
				float64(s.counters.operationsTotal) / float64(s.timers.durationTotal.Seconds()),
				float64(s.counters.sizeTotal) / float64(s.timers.durationTotal.Seconds()),
				float64(s.counters.errorsTotal) / float64(s.timers.durationTotal.Seconds()),
			},
			expectedMetricType: []model.MetricType{
				model.MetricTypeThroughput,
				model.MetricTypeThroughput,
				model.MetricTypeThroughput,
			},
			zeroValue: func() {
				original := s.timers.durationTotal
				s.timers.durationTotal = time.Duration(0)
				rollups := s.throughputs()
				s.timers.durationTotal = original
				assert.Equal(t, rollups, []model.PerfRollupValue{})
			},
		},
		{
			name:     "TestPercentiles",
			function: (*performanceStatistics).percentiles,
			expectedValue: []interface{}{
				stats.Sample{Xs: expectedExtractedDurations}.Quantile(0.5),
				stats.Sample{Xs: expectedExtractedDurations}.Quantile(0.8),
				stats.Sample{Xs: expectedExtractedDurations}.Quantile(0.9),
				stats.Sample{Xs: expectedExtractedDurations}.Quantile(0.95),
				stats.Sample{Xs: expectedExtractedDurations}.Quantile(0.99),
			},
			expectedMetricType: []model.MetricType{
				model.MetricTypePercentile50,
				model.MetricTypePercentile80,
				model.MetricTypePercentile90,
				model.MetricTypePercentile95,
				model.MetricTypePercentile99,
			},
			zeroValue: func() {
				original := s.timers.extractedDurations
				s.timers.extractedDurations = nil
				rollups := s.percentiles()
				s.timers.extractedDurations = original
				require.Empty(t, rollups)
			},
		},
		{
			name:     "TestSums",
			function: (*performanceStatistics).sums,
			expectedValue: []interface{}{
				s.timers.durationTotal,
				s.counters.errorsTotal,
				s.counters.operationsTotal,
				s.counters.sizeTotal,
			},
			expectedMetricType: []model.MetricType{
				model.MetricTypeSum,
				model.MetricTypeSum,
				model.MetricTypeSum,
				model.MetricTypeSum,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			actual := test.function(s)
			require.Equal(t, len(test.expectedValue), len(actual))
			for i, rollup := range actual {
				assert.Equal(t, test.expectedValue[i], rollup.Value)
				assert.Equal(t, test.expectedMetricType[i], rollup.MetricType)
				assert.False(t, rollup.UserSubmitted)
			}
			if test.zeroValue != nil {
				test.zeroValue()
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
