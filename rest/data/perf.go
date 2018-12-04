package data

import (
	"fmt"
	"net/http"

	"github.com/evergreen-ci/gimlet"
	"github.com/evergreen-ci/cedar/model"
	dataModel "github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/cedar/util"
)

// FindPerformanceResultById queries the database to find a given performance
// result.
func (prc *DBConnector) FindPerformanceResultById(id string) (*dataModel.APIPerformanceResult, error) {
	result := model.PerformanceResult{}
	result.Setup(prc.env)
	result.ID = id

	if err := result.Find(); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("performance result with id '%s' not found", id),
		}
	}

	apiResult := dataModel.APIPerformanceResult{}
	err := apiResult.Import(result)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("corrupt data"),
		}
	}
	return &apiResult, nil
}

// FindPerformanceResultsByTaskId queries the database to find all performance
// results with the given TaskID, time inteval, and optional tags.
func (prc *DBConnector) FindPerformanceResultsByTaskId(taskId string, interval util.TimeRange, tags ...string) ([]dataModel.APIPerformanceResult, error) {
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
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("database error"),
		}
	}
	if results.IsNil() {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("performance results with task_id '%s' not found", taskId),
		}
	}

	apiResults := make([]dataModel.APIPerformanceResult, len(results.Results))
	for i, result := range results.Results {
		err := apiResults[i].Import(result)
		if err != nil {
			return nil, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    fmt.Sprintf("corrupt data"),
			}
		}
	}
	return apiResults, nil
}

// FindPerformanceResultsByTaskId queries the database to find all performance
// results with the given version, time inteval, and optional tags.
func (prc *DBConnector) FindPerformanceResultsByVersion(version string, interval util.TimeRange, tags ...string) ([]dataModel.APIPerformanceResult, error) {
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
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("database error"),
		}
	}
	if results.IsNil() {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("performance results with version '%s' not found", version),
		}
	}

	apiResults := make([]dataModel.APIPerformanceResult, len(results.Results))
	for i, result := range results.Results {
		err := apiResults[i].Import(result)
		if err != nil {
			return nil, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    fmt.Sprintf("corrupt data"),
			}
		}
	}
	return apiResults, nil
}

// FindPerformanceResultsByTaskId queries the database to find a performance
// result, based on its id, and its children up to maxDepth and filtered by the
// optional tags.
func (prc *DBConnector) FindPerformanceResultWithChildren(id string, maxDepth int, tags ...string) ([]dataModel.APIPerformanceResult, error) {
	results := model.PerformanceResults{}
	results.Setup(prc.env)

	options := model.PerfFindOptions{
		Info: model.PerformanceResultInfo{
			Parent: id,
			Tags:   tags,
		},
		MaxDepth:    maxDepth,
		GraphLookup: true,
	}

	if err := results.Find(options); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("database error"),
		}
	}
	if results.IsNil() {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("performance result with id '%s' not found", id),
		}
	}

	apiResults := make([]dataModel.APIPerformanceResult, len(results.Results))
	for i, result := range results.Results {
		err := apiResults[i].Import(result)
		if err != nil {
			return nil, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    fmt.Sprintf("corrupt data"),
			}
		}
	}
	return apiResults, nil
}
