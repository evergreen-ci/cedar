package internal

import (
	"time"

	"github.com/evergreen-ci/sink/model"
	"github.com/golang/protobuf/ptypes"
	"github.com/pkg/errors"
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
	return model.CreatePerformanceResult(*m.Id.Export(), m.SourcePath, time.Duration(m.SampleRateDur))
}

func (m *MetricsPoint) Export() (model.PerformancePoint, error) {
	dur, err := ptypes.Duration(m.Duration)
	if err != nil {
		return model.PerformancePoint{}, errors.Wrap(err, "problem converting duration value")
	}
	ts, err := ptypes.Timestamp(m.Ts)
	if err != nil {
		return model.PerformancePoint{}, errors.Wrap(err, "problem converting duration value")
	}
	return model.PerformancePoint{
		Size:      m.Size,
		Count:     m.Count,
		Workers:   m.Workers,
		Duration:  dur,
		Timestamp: ts,
	}, nil
}
