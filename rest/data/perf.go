package data

import (
	"context"
	"net/http"
	"sort"
	"time"

	"github.com/evergreen-ci/cedar/model"
	dataModel "github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/cedar/units"
	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/amboy"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

/////////////////////////////
// DBConnector Implementation
/////////////////////////////

// FindPerformanceResultById queries the DB to find the performance
// result with the given ID.
func (dbc *DBConnector) FindPerformanceResultById(ctx context.Context, id string) (*dataModel.APIPerformanceResult, error) {
	result := model.PerformanceResult{}
	result.Setup(dbc.env)
	result.ID = id

	if err := result.Find(ctx); err != nil {
		if db.ResultsNotFound(err) {
			return nil, gimlet.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Message:    "not found",
			}
		}
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "DB error").Error(),
		}
	}

	apiResult := dataModel.APIPerformanceResult{}
	err := apiResult.Import(result)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "corrupt data").Error(),
		}
	}
	return &apiResult, nil
}

// RemovePerformanceResultById removes the performance result with the given ID
// from the DB. Note that this function deletes all children. No error is
// returned if the ID does not exist.
func (dbc *DBConnector) RemovePerformanceResultById(ctx context.Context, id string) (int, error) {
	result := model.PerformanceResult{}
	result.Setup(dbc.env)
	result.ID = id

	numRemoved, err := result.Remove(ctx)
	if err != nil {
		return numRemoved, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    "failed to remove performance result from DB",
		}
	}
	return numRemoved, nil
}

// FindPerformanceResults queries the DB to find all performance results
// that match the given options. If querying by task name, results are sorted
// (descending) by the Evergreen order.
func (dbc *DBConnector) FindPerformanceResults(ctx context.Context, opts PerformanceOptions) ([]dataModel.APIPerformanceResult, error) {
	var results model.PerformanceResults
	results.Setup(dbc.env)

	findOpts := model.PerfFindOptions{
		Info: model.PerformanceResultInfo{
			Project:   opts.Project,
			Version:   opts.Version,
			Variant:   opts.Variant,
			TaskID:    opts.TaskID,
			Execution: opts.Execution,
			TaskName:  opts.TaskName,
			Tags:      opts.Tags,
		},
		Interval: opts.Interval,
		Limit:    opts.Limit,
		Skip:     opts.Skip,
	}
	if opts.TaskName != "" {
		findOpts.Sort = []string{"info.order"}
	}

	if err := results.Find(ctx, findOpts); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "DB error").Error(),
		}
	}
	if results.IsNil() {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    "not found",
		}
	}

	apiResults := make([]dataModel.APIPerformanceResult, len(results.Results))
	for i, result := range results.Results {
		err := apiResults[i].Import(result)
		if err != nil {
			return nil, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    errors.Wrap(err, "corrupt data").Error(),
			}
		}
	}
	return apiResults, nil
}

// FindPerformanceResultWithChildren queries the DB to find a performance
// result with the given ID and its children up to maxDepth and filtered by
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
			Message:    errors.Wrap(err, "DB error").Error(),
		}
	}
	if results.IsNil() {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    "not found",
		}
	}

	apiResults := make([]dataModel.APIPerformanceResult, len(results.Results))
	for i, result := range results.Results {
		err := apiResults[i].Import(result)
		if err != nil {
			return nil, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    errors.Wrap(err, "corrupt data").Error(),
			}
		}
	}
	return apiResults, nil
}

// ScheduleSignalProcessingRecalculateJobs schedules signal processing recalculation jobs for
// each project/version/task/test/args combination.
func (dbc *DBConnector) ScheduleSignalProcessingRecalculateJobs(ctx context.Context) error {
	queue := dbc.env.GetRemoteQueue()
	allSeries, err := model.GetAllPerformanceResultSeriesIDs(ctx, dbc.env)
	if err != nil {
		return gimlet.ErrorResponse{StatusCode: http.StatusInternalServerError, Message: errors.Wrap(err, "getting time series IDs").Error()}
	}

	catcher := grip.NewBasicCatcher()

	for _, series := range allSeries {
		job := units.NewUpdateTimeSeriesJob(series)
		if err := amboy.EnqueueUniqueJob(ctx, queue, job); err != nil {
			catcher.Add(message.WrapError(err, message.Fields{
				"message": "unable to enqueue recalculation job for metric",
				"project": series.Project,
				"variant": series.Variant,
				"task":    series.Task,
				"test":    series.Test,
				"args":    series.Arguments,
			}))
		}
	}
	return catcher.Resolve()
}

///////////////////////////////
// MockConnector Implementation
///////////////////////////////

// FindPerformanceResultById queries the mock cache to find the performance
// result with the given ID.
func (mc *MockConnector) FindPerformanceResultById(_ context.Context, id string) (*dataModel.APIPerformanceResult, error) {
	result, ok := mc.CachedPerformanceResults[id]
	apiResult := dataModel.APIPerformanceResult{}
	err := apiResult.Import(result)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "corrupt data").Error(),
		}
	}

	if !ok {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    "not found",
		}
	}
	return &apiResult, nil
}

// RemovePerformanceResultById removes the performance result with the given ID
// from the mock cache. Note that this function deletes all children. No error
// is returned if the ID does not exist.
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

// FindPerformanceResults finds the performance results matching the given
// options.
func (mc *MockConnector) FindPerformanceResults(_ context.Context, opts PerformanceOptions) ([]dataModel.APIPerformanceResult, error) {
	var results []dataModel.APIPerformanceResult
	for _, result := range mc.CachedPerformanceResults {
		if opts.Project != "" && opts.Project != result.Info.Project {
			continue
		}
		if opts.Version != "" && opts.Version != result.Info.Version {
			continue
		}
		if opts.Variant != "" && opts.Variant != result.Info.Variant {
			continue
		}
		if opts.TaskID != "" && opts.TaskID != result.Info.TaskID {
			continue
		}
		if opts.TaskName != "" && opts.TaskName != result.Info.TaskName {
			continue
		}
		if opts.Execution != 0 && opts.Execution != result.Info.Execution {
			continue
		}
		if opts.TaskID == "" && opts.Version == "" {
			if !result.Info.Mainline {
				continue
			}
			if !mc.checkInterval(result.ID, opts.Interval) {
				continue
			}
		}
		if !containsTags(opts.Tags, result.Info.Tags) {
			continue
		}

		var apiResult dataModel.APIPerformanceResult
		if err := apiResult.Import(result); err != nil {
			return nil, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    errors.Wrap(err, "corrupt data").Error(),
			}
		}

		results = append(results, apiResult)
	}
	if len(results) == 0 {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    "not found",
		}
	}

	if opts.TaskName != "" {
		sort.Slice(results, func(i, j int) bool { return results[i].Info.Order > results[j].Info.Order })
	} else {
		sort.Slice(results, func(i, j int) bool { return time.Time(results[i].CreatedAt).After(time.Time(results[j].CreatedAt)) })
	}

	if opts.Skip > 0 && opts.Skip < len(results) {
		results = results[opts.Skip:]
	}
	if opts.Limit > 0 && opts.Limit < len(results) {
		results = results[:opts.Limit]
	}

	return results, nil
}

// FindPerformanceResultWithChildren queries the mock cache to find a
// performance result with the given ID and its children up to maxDepth and
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
						Message:    errors.Wrap(err, "corrupt data").Error(),
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
