package data

import (
	"context"
	"net/http"

	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/gimlet"
	"github.com/pkg/errors"
)

/////////////////////////////
// DBConnector Implementation
/////////////////////////////

func (dbc *DBConnector) FindTestResults(ctx context.Context, taskOpts []TestResultsTaskOptions, filterOpts *TestResultsFilterAndSortOptions) (*model.APITestResults, error) {
	dbFilterOpts, err := convertToDBTestResultsFilterAndSortOptions(filterOpts)
	if err != nil {
		return nil, err
	}
	stats, results, err := dbModel.FindAndDownloadTestResults(ctx, dbc.env, convertToDBTestResultsTaskOptions(taskOpts), dbFilterOpts)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "retrieving test results").Error(),
		}
	}

	apiStats := &model.APITestResultsStats{}
	if err = apiStats.Import(stats); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "importing stats into APITestResultsStats struct").Error(),
		}
	}

	apiResults, err := importTestResults(ctx, results)
	if err != nil {
		return nil, err
	}

	return &model.APITestResults{
		Stats:   *apiStats,
		Results: apiResults,
	}, nil
}

func (dbc *DBConnector) FindTestResultsStats(ctx context.Context, opts []TestResultsTaskOptions) (*model.APITestResultsStats, error) {
	stats, err := dbModel.FindTestResultsStats(ctx, dbc.env, convertToDBTestResultsTaskOptions(opts))
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "retrieving test results stats").Error(),
		}
	}

	apiStats := &model.APITestResultsStats{}
	if err = apiStats.Import(stats); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "importing stats into APITestResultsStats struct").Error(),
		}
	}

	return apiStats, nil
}

func (dbc *DBConnector) FindFailedTestResultsSample(ctx context.Context, opts []TestResultsTaskOptions) ([]string, error) {
	resultDocs, err := dbModel.FindTestResults(ctx, dbc.env, convertToDBTestResultsTaskOptions(opts))
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "retrieving test results").Error(),
		}
	}

	return extractFailedTestResultsSample(resultDocs...), nil
}

func (dbc *DBConnector) FindFailedTestResultsSamples(ctx context.Context, taskOpts []TestResultsTaskOptions, regexFilters []string) ([]model.APITestResultsSample, error) {
	samples, err := dbModel.FindFailedTestResultsSamples(ctx, dbc.env, convertToDBTestResultsTaskOptions(taskOpts), regexFilters)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "retrieving test results samples").Error(),
		}
	}

	return importTestResultsSamples(samples)
}

///////////////////////////////
// MockConnector Implementation
///////////////////////////////

func (mc *MockConnector) FindTestResults(_ context.Context, _ []TestResultsTaskOptions, _ *TestResultsFilterAndSortOptions) (*model.APITestResults, error) {
	return nil, errors.New("not implemented")
}

func (mc *MockConnector) FindTestResultsStats(_ context.Context, _ []TestResultsTaskOptions) (*model.APITestResultsStats, error) {
	return nil, errors.New("not implemented")
}

func (mc *MockConnector) FindFailedTestResultsSample(ctx context.Context, _ []TestResultsTaskOptions) ([]string, error) {
	return nil, errors.New("not implemented")
}
func (mc *MockConnector) FindFailedTestResultsSamples(ctx context.Context, _ []TestResultsTaskOptions, _ []string) ([]model.APITestResultsSample, error) {
	return nil, errors.New("not implemented")
}

///////////////////
// Helper Functions
///////////////////

func importTestResults(ctx context.Context, results []dbModel.TestResult) ([]model.APITestResult, error) {
	apiResults := []model.APITestResult{}

	for _, result := range results {
		apiResult := model.APITestResult{}
		err := apiResult.Import(result)
		if err != nil {
			return nil, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    errors.Wrapf(err, "importing result into APITestResult struct").Error(),
			}
		}
		apiResults = append(apiResults, apiResult)
	}

	if err := ctx.Err(); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    err.Error(),
		}
	}

	return apiResults, nil
}

func importTestResultsSamples(results []dbModel.TestResultsSample) ([]model.APITestResultsSample, error) {
	samples := make([]model.APITestResultsSample, 0, len(results))
	for _, result := range results {
		apiSample := model.APITestResultsSample{}
		if err := apiSample.Import(result); err != nil {
			return nil, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    errors.Wrapf(err, "importing sample into APITestResultsSample struct").Error(),
			}
		}
		samples = append(samples, apiSample)
	}
	return samples, nil
}

func extractFailedTestResultsSample(results ...dbModel.TestResults) []string {
	var sample []string
	for i := 0; i < len(results) && len(sample) < dbModel.FailedTestsSampleSize; i++ {
		sample = append(sample, results[i].FailedTestsSample...)
	}

	return sample
}

func convertToDBTestResultsTaskOptions(opts []TestResultsTaskOptions) []dbModel.TestResultsTaskOptions {
	if len(opts) == 0 {
		return nil
	}

	dbOpts := make([]dbModel.TestResultsTaskOptions, len(opts))
	for i, task := range opts {
		dbOpts[i].TaskID = task.TaskID
		dbOpts[i].Execution = task.Execution
	}

	return dbOpts
}

func convertToDBTestResultsSortOptions(opts []TestResultsSortBy) []dbModel.TestResultsSortBy {
	if len(opts) == 0 {
		return nil
	}

	dbOpts := make([]dbModel.TestResultsSortBy, len(opts))
	for i, sortBy := range opts {
		dbOpts[i].Key = sortBy.Key
		dbOpts[i].SortOrderDSC = sortBy.SortOrderDSC
	}

	return dbOpts
}

func convertToDBTestResultsFilterAndSortOptions(opts *TestResultsFilterAndSortOptions) (*dbModel.TestResultsFilterAndSortOptions, error) {
	if opts == nil {
		return nil, nil
	}

	dbOpts := &dbModel.TestResultsFilterAndSortOptions{
		TestName:  opts.TestName,
		Statuses:  opts.Statuses,
		GroupID:   opts.GroupID,
		Sort:      convertToDBTestResultsSortOptions(opts.Sort),
		Limit:     opts.Limit,
		Page:      opts.Page,
		BaseTasks: convertToDBTestResultsTaskOptions(opts.BaseTasks),
	}

	// TODO (EVG-14306): Remove this logic once Evergreen's GraphQL service
	// is no longer using these fields.
	if opts.SortBy != "" {
		dbOpts.Sort = []dbModel.TestResultsSortBy{
			{
				Key:          opts.SortBy,
				SortOrderDSC: opts.SortOrderDSC,
			},
		}
	}

	if err := dbOpts.Validate(); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    errors.Wrap(err, "invalid filter options").Error(),
		}
	}

	return dbOpts, nil
}
