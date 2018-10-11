package internal

import (
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
	panic("not implemented")
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

	point := model.PerformancePoint{
		Timestamp: ts,
	}

	point.Counters.Size = m.Size
	point.Counters.Operations = m.Count
	point.State.Workers = m.Workers
	point.Timers.Duration = dur

	return point, nil
}
