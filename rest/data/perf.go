package data

import (
	"time"

	"github.com/evergreen-ci/gimlet"
	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/model"
	"github.com/evergreen-ci/sink/util"
)

// DBPerformanceResultConnector is a struct that implements the Perf
// related from the Connector through interactions with the backing database.
type DBPerformanceResultConnector struct{}

type DBPerformanceResultInput struct {
	Id        string
	TaskId    string
	Versison  int
	Project   string
	TaskName  string
	TestName  string
	Tags      []string
	MaxDepth  int
	TimeRange util.TimeRange
	Env       sink.Environment
}

func (prc *DBPerformanceResultConnector) FindPerformanceResultById(input DBPerformanceResultInput) (*PerformanceResult, error) {
	result := &model.PerformanceResult{}
	result.Setup(prc.Env)
	result.ID = input.Id

	if err := result.Find(); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("performance result with id '%s' not found", input.Id),
		}
	}
	return result, nil
}
