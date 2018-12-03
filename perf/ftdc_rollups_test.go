package perf

import (
	"bytes"
	"context"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/evergreen-ci/sink/model"
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
				assert.Equal(t, 13, len(actual))
				assert.NoError(t, err)
			}
		})
	}
}

func TestCalcFunctions(t *testing.T) {
	s := &performanceStatistics{
		counters: struct {
			operations int64
			size       int64
			errors     int64
		}{
			operations: 10,
			size:       1000,
			errors:     5,
		},
		timers: struct {
			durationTotal time.Duration
			total         time.Duration
		}{
			durationTotal: time.Duration(5000000),
			total:         time.Duration(6000000),
		},
		gauges: struct {
			stateTotal   int64
			workersTotal int64
			failedTotal  int64
		}{
			stateTotal:   90,
			workersTotal: 100,
			failedTotal:  2,
		},
		numSamples: 98,
	}

	for _, test := range []struct {
		name       string
		function   func(*performanceStatistics) []model.PerfRollupValue
		expected   []interface{}
		metricType model.MetricType
		zeroValue  func()
	}{
		{
			name:     "TestMeans",
			function: (*performanceStatistics).perfMeans,
			expected: []interface{}{
				float64(s.timers.durationTotal) / float64(s.numSamples),
				float64(s.gauges.stateTotal) / float64(s.numSamples),
				float64(s.gauges.workersTotal) / float64(s.numSamples),
			},
			metricType: model.MetricTypeMean,
			zeroValue: func() {
				original := s.numSamples
				s.numSamples = 0
				rollups := s.perfMeans()
				s.numSamples = original
				assert.Equal(t, rollups, []model.PerfRollupValue{})
			},
		},
		{
			name:     "TestThroughputs",
			function: (*performanceStatistics).perfThroughputs,
			expected: []interface{}{
				float64(s.counters.operations) / float64(s.timers.durationTotal.Seconds()),
				float64(s.counters.size) / float64(s.timers.durationTotal.Seconds()),
				float64(s.counters.errors) / float64(s.timers.durationTotal.Seconds()),
			},
			metricType: model.MetricTypeThroughput,
			zeroValue: func() {
				original := s.timers.durationTotal
				s.timers.durationTotal = time.Duration(0)
				rollups := s.perfThroughputs()
				s.timers.durationTotal = original
				assert.Equal(t, rollups, []model.PerfRollupValue{})
			},
		},
		{
			name:     "TestLatencies",
			function: (*performanceStatistics).perfLatencies,
			expected: []interface{}{
				float64(s.timers.durationTotal) / float64(s.counters.operations),
			},
			metricType: model.MetricTypeLatency,
			zeroValue: func() {
				original := s.counters.operations
				s.counters.operations = 0
				rollups := s.perfLatencies()
				s.counters.operations = original
				require.Equal(t, 1, len(rollups))
				assert.Equal(t, rollups[0].Value, math.Inf(0))
			},
		},
		{
			name:     "TestTotals",
			function: (*performanceStatistics).perfTotals,
			expected: []interface{}{
				s.timers.durationTotal,
				s.gauges.failedTotal,
				s.counters.errors,
				s.counters.operations,
				s.counters.size,
				s.numSamples,
			},
			metricType: model.MetricTypeSum,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			actual := test.function(s)
			require.Equal(t, len(test.expected), len(actual))
			for i, rollup := range actual {
				assert.Equal(t, test.expected[i], rollup.Value)
				assert.Equal(t, test.metricType, rollup.MetricType)
			}
			if test.zeroValue != nil {
				test.zeroValue()
			}
		})
	}
}

func createFTDC(valid bool, samples int) ([]bytes, error) {
	collector := ftdc.NewDynamicCollector(5)
	if valid {
		recorder := events.NewRawRecorder(collector)

		for i := 0; i < samples; i++ {
			recorder.Begin()
			recorder.IncOps(rand.Int63n(100) + 1)
			recorder.End(time.Duration(1000000))
		}
		if err := recorder.Flush(); err != nil {
			return []bytes{}, errors.WithStack(err)
		}

	} else {
		for i := 0; i < samples; i++ {
			err := collector.Add(bsonx.NewDocument(bsonx.EC.Int64("one", rand.Int63()), bsonx.EC.Int64("two", rand.Int63())))
			if err != nil {
				return []bytes{}, errors.WithStack(err)
			}
		}
	}

	data, err := collector.Resolve()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}
