package data

import (
	"fmt"
	"net/http"
	"time"

	"github.com/evergreen-ci/cedar/model"
	dataModel "github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/gimlet"
)

// FindPerformanceResultById queries the database to find a given performance
// result.
func (dbc *DBConnector) FindPerformanceResultById(id string) (*dataModel.APIPerformanceResult, error) {
	result := model.PerformanceResult{}
	result.Setup(dbc.env)
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
func (dbc *DBConnector) FindPerformanceResultsByTaskId(taskId string, interval util.TimeRange, tags ...string) ([]dataModel.APIPerformanceResult, error) {
	results := model.PerformanceResults{}
	results.Setup(dbc.env)

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
func (dbc *DBConnector) FindPerformanceResultsByVersion(version string, interval util.TimeRange, tags ...string) ([]dataModel.APIPerformanceResult, error) {
	results := model.PerformanceResults{}
	results.Setup(dbc.env)

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
func (dbc *DBConnector) FindPerformanceResultWithChildren(id string, maxDepth int, tags ...string) ([]dataModel.APIPerformanceResult, error) {
	results := model.PerformanceResults{}
	results.Setup(dbc.env)

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

// MockConnector Implementation

func (mc *MockConnector) FindPerformanceResultById(id string) (*dataModel.APIPerformanceResult, error) {
	result, ok := mc.CachedPerformanceResults[id]
	if !ok {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("performance result with id '%s' not found", id),
		}
	}
	return &result, nil
}

func (mc *MockConnector) FindPerformanceResultsByTaskId(taskId string, interval util.TimeRange, tags ...string) ([]dataModel.APIPerformanceResult, error) {
	results := []dataModel.APIPerformanceResult{}
	for _, result := range mc.CachedPerformanceResults {
		if *result.Info.TaskID == taskId && mc.checkInterval(*result.Name, interval) && mc.checkTags(*result.Name, tags) {
			results = append(results, result)
		}
	}

	if len(results) == 0 {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("performance result with task_id '%s' not found", taskId),
		}
	}
	return results, nil
}

func (mc *MockConnector) FindPerformanceResultsByVersion(version string, interval util.TimeRange, tags ...string) ([]dataModel.APIPerformanceResult, error) {
	results := []dataModel.APIPerformanceResult{}
	for _, result := range mc.CachedPerformanceResults {
		if *result.Info.Version == version && mc.checkInterval(*result.Name, interval) && mc.checkTags(*result.Name, tags) {
			results = append(results, result)
		}
	}

	if len(results) == 0 {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("performance result with version '%s' not found", version),
		}
	}
	return results, nil
}

func (mc *MockConnector) FindPerformanceResultWithChildren(id string, maxDepth int, tags ...string) ([]dataModel.APIPerformanceResult, error) {
	results := []dataModel.APIPerformanceResult{}

	result, err := mc.FindPerformanceResultById(id)
	if err != nil {
		return nil, err
	}
	results = append(results, *result)

	return append(results, mc.findChildren(id, maxDepth, tags)...), nil
}

func (mc *MockConnector) checkInterval(id string, interval util.TimeRange) bool {
	result, _ := mc.CachedPerformanceResults[id]
	createdAt := time.Time(result.CreatedAt)
	completedAt := time.Time(result.CompletedAt)
	return (interval.StartAt.Before(createdAt) || interval.StartAt.Equal(time.Time(createdAt))) &&
		(interval.EndAt.After(completedAt) || interval.EndAt.Equal(completedAt))
}

func (mc *MockConnector) checkTags(id string, tags []string) bool {
	result, _ := mc.CachedPerformanceResults[id]
	tagMap := make(map[string]bool)
	for _, tag := range result.Info.Tags {
		tagMap[tag] = true
	}

	for _, tag := range tags {
		_, ok := tagMap[tag]
		if !ok {
			return false
		}
	}
	return true
}

func (mc *MockConnector) findChildren(id string, maxDepth int, tags []string) []dataModel.APIPerformanceResult {
	results := []dataModel.APIPerformanceResult{}
	seen := map[string]int{id: 0}
	queue := []string{id}

	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		if seen[next] >= maxDepth {
			continue
		}
		children, _ := mc.ChildMap[next]
		queue = append(queue, children...)
		for _, child := range children {
			seen[child] = seen[next] + 1
			result, ok := mc.CachedPerformanceResults[child]
			if ok && mc.checkTags(child, tags) {
				results = append(results, result)
			}
		}
	}
	return results
}
