package data

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/pkg/errors"

	"github.com/evergreen-ci/cedar/model"
	dataModel "github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/cedar/units"
	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/amboy"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
)

/////////////////////////////
// DBConnector Implementation
/////////////////////////////

// FindPerformanceResultById queries the database to find the performance
// result with the given id.
func (dbc *DBConnector) FindPerformanceResultById(ctx context.Context, id string) (*dataModel.APIPerformanceResult, error) {
	result := model.PerformanceResult{}
	result.Setup(dbc.env)
	result.ID = id

	if err := result.Find(ctx); err != nil {
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

// RemovePerformanceResultById removes the performance result with the given id
// from the database. Note that this function deletes all children. No error is
// returned if the id does not exist.
func (dbc *DBConnector) RemovePerformanceResultById(ctx context.Context, id string) (int, error) {
	result := model.PerformanceResult{}
	result.Setup(dbc.env)
	result.ID = id

	numRemoved, err := result.Remove(ctx)
	if err != nil {
		return numRemoved, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    "failed to remove performance result from database",
		}
	}
	return numRemoved, nil
}

// FindPerformanceResultsByTaskId queries the database to find all performance
// results with the given taskId and that fall within interval, filtered by
// tags.
func (dbc *DBConnector) FindPerformanceResultsByTaskId(ctx context.Context, taskId string, interval model.TimeRange, tags ...string) ([]dataModel.APIPerformanceResult, error) {
	results := model.PerformanceResults{}
	results.Setup(dbc.env)

	options := model.PerfFindOptions{
		Interval: interval,
		Info: model.PerformanceResultInfo{
			TaskID: taskId,
			Tags:   tags,
		},
		MaxDepth: 0,
	}

	if err := results.Find(ctx, options); err != nil {
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

// FindPerformanceResultsByTaskName queries the database to find all
// performance results with the given taskName and that fall within interval,
// filtered by tags. Results are returned sorted (descending) by the Evergreen
// order. If limit is greater than 0, the number of results returned will be no
// greater than limit.
func (dbc *DBConnector) FindPerformanceResultsByTaskName(ctx context.Context, project, taskName, variant string, interval model.TimeRange, limit int, tags ...string) ([]dataModel.APIPerformanceResult, error) {
	results := model.PerformanceResults{}
	results.Setup(dbc.env)

	options := model.PerfFindOptions{
		Interval: interval,
		Info: model.PerformanceResultInfo{
			TaskName: taskName,
			Tags:     tags,
			Project:  project,
		},
		MaxDepth: 0,
		Limit:    limit,
		Variant:  variant,
		Sort:     []string{"info.order"},
	}

	if err := results.Find(ctx, options); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("database error"),
		}
	}
	if results.IsNil() {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("performance results with task_name '%s' not found", taskName),
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

// FindPerformanceResultsByVersion queries the database to find all performance
// results with the given version and that fall within interval, filtered by
// tags.
func (dbc *DBConnector) FindPerformanceResultsByVersion(ctx context.Context, version string, interval model.TimeRange, tags ...string) ([]dataModel.APIPerformanceResult, error) {
	results := model.PerformanceResults{}
	results.Setup(dbc.env)

	options := model.PerfFindOptions{
		Interval: interval,
		Info: model.PerformanceResultInfo{
			Version: version,
			Tags:    tags,
		},
		MaxDepth: 0,
	}

	if err := results.Find(ctx, options); err != nil {
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

// FindPerformanceResultWithChildren queries the database to find a performance
// result with the given id and its children up to maxDepth and filtered by
// tags. If maxDepth is less than 0, the child search is exhaustive.
func (dbc *DBConnector) FindPerformanceResultWithChildren(ctx context.Context, id string, maxDepth int, tags ...string) ([]dataModel.APIPerformanceResult, error) {
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

	if err := results.Find(ctx, options); err != nil {
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

// ScheduleSignalProcessingRecalculateJobs schedules signal processing recalculation jobs for
// each project/version/task/test combination
func (dbc *DBConnector) ScheduleSignalProcessingRecalculateJobs(ctx context.Context) error {
	queue := dbc.env.GetRemoteQueue()
	ids, err := model.GetPerformanceResultSeriesIDs(ctx, dbc.env)
	if err != nil {
		return gimlet.ErrorResponse{StatusCode: http.StatusInternalServerError, Message: fmt.Sprint("Failed to get time series ids")}
	}

	catcher := grip.NewBasicCatcher()

	for _, id := range ids {
		job := units.NewRecalculateChangePointsJob(id)
		err := amboy.EnqueueUniqueJob(ctx, queue, job)
		if err != nil {
			catcher.Add(errors.New(message.WrapError(err, message.Fields{
				"message": "Unable to enqueue recalculation job for metric",
				"project": id.Project,
				"variant": id.Variant,
				"task":    id.Task,
				"test":    id.Test,
			}).String()))
		}
	}
	return catcher.Resolve()
}

func (dbc *DBConnector) MarkChangePoints(ctx context.Context, status string, ) error {
	model.SetTriageStatus()
}

///////////////////////////////
// MockConnector Implementation
///////////////////////////////

// FindPerformanceResultById queries the mock cache to find the performance
// result with the given id.
func (mc *MockConnector) FindPerformanceResultById(_ context.Context, id string) (*dataModel.APIPerformanceResult, error) {
	result, ok := mc.CachedPerformanceResults[id]
	apiResult := dataModel.APIPerformanceResult{}
	err := apiResult.Import(result)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("corrupt data"),
		}
	}

	if !ok {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("performance result with id '%s' not found", id),
		}
	}
	return &apiResult, nil
}

// RemovePerformanceResultById removes the performance result with the given id
// from the mock cache. Note that this function deletes all children. No error
// is returned if the id does not exist.
func (mc *MockConnector) RemovePerformanceResultById(_ context.Context, id string) (int, error) {
	_, ok := mc.CachedPerformanceResults[id]
	if ok {
		delete(mc.CachedPerformanceResults, id)
	} else {
		return 0, nil
	}

	children, err := mc.findChildren(id, -1, []string{})
	for _, child := range children {
		delete(mc.CachedPerformanceResults, *child.Name)
	}
	return len(children) + 1, err
}

// FindPerformanceResultsByTaskId queries the mock cache to find all
// performance results with the given taskId and that fall within interval,
// filtered by tags.
func (mc *MockConnector) FindPerformanceResultsByTaskId(_ context.Context, taskId string, interval model.TimeRange, tags ...string) ([]dataModel.APIPerformanceResult, error) {
	results := []dataModel.APIPerformanceResult{}
	for _, result := range mc.CachedPerformanceResults {
		if result.Info.TaskID == taskId && mc.checkInterval(result.ID, interval) && containsTags(tags, result.Info.Tags) {
			apiResult := dataModel.APIPerformanceResult{}
			err := apiResult.Import(result)
			if err != nil {
				return nil, gimlet.ErrorResponse{
					StatusCode: http.StatusInternalServerError,
					Message:    fmt.Sprintf("corrupt data"),
				}
			}

			results = append(results, apiResult)
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

// FindPerformanceResultsByTaskName queries the mock cache to find all
// performance results with the given taskName and that fall within interval,
// filtered by tags. Results are returned sorted (descending) by the Evergreen
// order. If limit is greater than 0, the number of results returned will be no
// greater than limit.
func (mc *MockConnector) FindPerformanceResultsByTaskName(_ context.Context, project, taskName, variant string, interval model.TimeRange, limit int, tags ...string) ([]dataModel.APIPerformanceResult, error) {
	results := []dataModel.APIPerformanceResult{}
	for _, result := range mc.CachedPerformanceResults {
		if result.Info.TaskName == taskName && mc.checkInterval(result.ID, interval) && containsTags(tags, result.Info.Tags) &&
			result.Info.Mainline && (variant == "" || result.Info.Variant == variant) {
			apiResult := dataModel.APIPerformanceResult{}
			err := apiResult.Import(result)
			if err != nil {
				return nil, gimlet.ErrorResponse{
					StatusCode: http.StatusInternalServerError,
					Message:    fmt.Sprintf("corrupt data"),
				}
			}
			results = append(results, apiResult)
		}
	}

	if len(results) == 0 {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("performance result with task_name '%s' not found", taskName),
		}
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Info.Order > results[j].Info.Order })
	if limit > 0 && limit < len(results) {
		results = results[:limit]
	}

	return results, nil
}

// FindPerformanceResultsByVersion queries the mock cache to find all
// performance results with the given version and that fall within interval,
// filtered by tags.
func (mc *MockConnector) FindPerformanceResultsByVersion(_ context.Context, version string, interval model.TimeRange, tags ...string) ([]dataModel.APIPerformanceResult, error) {
	results := []dataModel.APIPerformanceResult{}
	for _, result := range mc.CachedPerformanceResults {
		if result.Info.Version == version && mc.checkInterval(result.ID, interval) && containsTags(tags, result.Info.Tags) && result.Info.Mainline {
			apiResult := dataModel.APIPerformanceResult{}
			err := apiResult.Import(result)
			if err != nil {
				return nil, gimlet.ErrorResponse{
					StatusCode: http.StatusInternalServerError,
					Message:    fmt.Sprintf("corrupt data"),
				}
			}

			results = append(results, apiResult)
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

// FindPerformanceResultWithChildren queries the mock cache to find a
// performance result with the given id and its children up to maxDepth and
// filtered by tags. If maxDepth is less than 0, the child search is
// exhaustive.
func (mc *MockConnector) FindPerformanceResultWithChildren(ctx context.Context, id string, maxDepth int, tags ...string) ([]dataModel.APIPerformanceResult, error) {
	results := []dataModel.APIPerformanceResult{}

	result, err := mc.FindPerformanceResultById(ctx, id)
	if err != nil {
		return nil, err
	}
	results = append(results, *result)

	children, err := mc.findChildren(id, maxDepth, tags)
	return append(results, children...), err
}

func (mc *MockConnector) checkInterval(id string, interval model.TimeRange) bool {
	result := mc.CachedPerformanceResults[id]
	createdAt := result.CreatedAt
	completedAt := result.CompletedAt
	return (interval.StartAt.Before(createdAt) || interval.StartAt.Equal(createdAt)) &&
		(interval.EndAt.After(completedAt) || interval.EndAt.Equal(completedAt))
}

func (mc *MockConnector) findChildren(id string, maxDepth int, tags []string) ([]dataModel.APIPerformanceResult, error) {
	if maxDepth == 0 {
		return []dataModel.APIPerformanceResult{}, nil
	}

	results := []dataModel.APIPerformanceResult{}
	seen := map[string]int{id: 1}
	queue := []string{id}

	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		if seen[next] > maxDepth && maxDepth > 0 {
			continue
		}
		children := mc.ChildMap[next]
		queue = append(queue, children...)
		for _, child := range children {
			seen[child] = seen[next] + 1
			result, ok := mc.CachedPerformanceResults[child]
			if ok && containsTags(tags, result.Info.Tags) {
				apiResult := dataModel.APIPerformanceResult{}
				err := apiResult.Import(result)
				if err != nil {
					return nil, gimlet.ErrorResponse{
						StatusCode: http.StatusInternalServerError,
						Message:    fmt.Sprintf("corrupt data"),
					}
				}

				results = append(results, apiResult)
			}
		}
	}
	return results, nil
}

// ScheduleSignalProcessingRecalculateJobs schedules signal processing recalculation jobs for
// each project/version/task/test combination
func (mc *MockConnector) ScheduleSignalProcessingRecalculateJobs(ctx context.Context) error {
	return nil
}
