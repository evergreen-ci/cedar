package internal

import (
	"time"

	"github.com/evergreen-ci/sink/model"
)

func (m *ResultID) Export() *model.PerformanceResultID {
	return &model.PerformanceResultID{
		TaskName:  m.TaskName,
		Execution: int(m.Execution),
		TestName:  m.TestName,
		Parent:    m.Parent,
		Tags:      m.Tags,
		Arguments: m.Arguments,
	}
}

func (m *MetricsSeriesStart) Export() *model.PerformanceResult {
	return model.CreatePerformanceResult(m.Id.Export(), m.SourcePath, time.Duration(m.SampleRateDur))
}

func (m *MetricsPoint) Export() model.PerformancePoint {
	return &model.PerformancePoint{
		Size:     m.Size,
		Count:    m.Count,
		Workers:  m.Workers,
		Duration: time.Duration(m.Duration),
	}
}
