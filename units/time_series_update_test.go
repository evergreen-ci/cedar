package units

import (
	"context"
	"testing"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/perf"
	"github.com/stretchr/testify/require"
)

type MockPerformanceAnalysisService struct {
	Calls   []perf.TimeSeriesModel
	Results [][]int
}

func (m *MockPerformanceAnalysisService) ReportUpdatedTimeSeries(ctx context.Context, timeSeries perf.TimeSeriesModel) error {
	m.Calls = append(m.Calls, timeSeries)
	return nil
}

func TestUpdateTimeSeriesJob(t *testing.T) {
	env := cedar.GetEnvironment()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("ReportsTimeSeries", func(t *testing.T) {
		_ = env.GetDB().Drop(ctx)
		series := model.UnanalyzedPerformanceSeries{
			Project:      "project",
			Variant:      "variant",
			Task:         "task",
			Test:         "test",
			Arguments:    map[string]int32{"thread_level": 20},
			Measurements: []string{"ops_per_sec", "avg", "latency"},
		}

		j := NewUpdateTimeSeriesJob(series)
		mockDetector := &MockPerformanceAnalysisService{}
		job := j.(*timeSeriesUpdateJob)
		job.service = mockDetector
		job.conf = model.NewCedarConfig(env)
		j.Run(ctx)
		require.True(t, j.Status().Completed)
		require.Equal(t, len(mockDetector.Calls), len(series.Measurements))
		for i, call := range mockDetector.Calls {
			require.Equal(t, series.Project, call.Project)
			require.Equal(t, series.Variant, call.Variant)
			require.Equal(t, series.Task, call.Task)
			require.Equal(t, series.Test, call.Test)
			require.Equal(t, []perf.ArgumentsModel{{Name: "thread_level", Value: int32(20)}}, call.Arguments)
			require.Equal(t, series.Measurements[i], call.Measurement)
		}
	})
	t.Run("DoesNothingWhenDisabled", func(t *testing.T) {
		j := NewUpdateTimeSeriesJob(model.UnanalyzedPerformanceSeries{
			Project: "projecta",
			Variant: "variant",
			Task:    "task",
			Test:    "test",
		})
		mockDetector := &MockPerformanceAnalysisService{}
		job := j.(*timeSeriesUpdateJob)
		job.service = mockDetector
		job.conf = model.NewCedarConfig(env)
		job.conf.Flags.DisableSignalProcessing = true
		j.Run(ctx)
		require.True(t, j.Status().Completed)
		require.Len(t, mockDetector.Calls, 0)
	})
}
