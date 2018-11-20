package data

import (
	"fmt"
	"net/http"

	"github.com/evergreen-ci/gimlet"
	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/model"
	"github.com/evergreen-ci/sink/util"
)

// DBPerformanceResultConnector is a struct that implements the Perf
// related from the Connector through interactions with the backing database.
type DBPerformanceResultConnector struct {
	env sink.Environment
}

func (prc *DBPerformanceResultConnector) FindPerformanceResultById(id string) (*model.PerformanceResult, error) {
	result := &model.PerformanceResult{}
	result.Setup(prc.env)
	result.ID = id

	if err := result.Find(); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("performance result with id '%s' not found", id),
		}
	}
	return result, nil
}

func (prc *DBPerformanceResultConnector) FindPerformanceResultsByTaskId(taskId string, interval util.TimeRange, tags ...string) ([]model.PerformanceResult, error) {
	results := model.PerformanceResults{}
	results.Setup(prc.env)

	options := model.PerfFindOptions{
		Interval: interval,
		Info: model.PerformanceResultInfo{
			TaskID: taskId,
			Tags:   tags,
		},
		MaxDepth: -1,
	}

	if err := results.Find(options); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("performance results with task_id '%s' not found", taskId),
		}
	}
	return results.Results, nil
}

func (prc *DBPerformanceResultConnector) FindPerformanceResultsByVersion(version string, interval util.TimeRange, tags ...string) ([]model.PerformanceResult, error) {
	results := model.PerformanceResults{}
	results.Setup(prc.env)

	options := model.PerfFindOptions{
		Interval: interval,
		Info: model.PerformanceResultInfo{
			Version: version,
			Tags:    tags,
		},
		MaxDepth: -1,
	}

	if err := results.Find(options); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("performance results with version '%s' not found", version),
		}
	}
	return results.Results, nil
}

func (prc *DBPerformanceResultConnector) FindPerformanceResultWithChildren(id string, interval util.TimeRange, maxDepth int, tags ...string) ([]model.PerformanceResult, error) {
	results := model.PerformanceResults{}
	results.Setup(prc.env)

	options := model.PerfFindOptions{
		Interval: interval,
		Info: model.PerformanceResultInfo{
			Parent: id,
			Tags:   tags,
		},
		MaxDepth: maxDepth,
	}

	if err := results.Find(options); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("performance result with id '%s' not found", id),
		}
	}
	return results.Results, nil
}
