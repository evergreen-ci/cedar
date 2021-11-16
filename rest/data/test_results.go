package data

import (
	"context"
	"net/http"

	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/gimlet"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/anser/db"
	"github.com/pkg/errors"
)

/////////////////////////////
// DBConnector Implementation
/////////////////////////////

func (dbc *DBConnector) FindTestResults(ctx context.Context, opts TestResultsOptions) (*model.APITestResults, error) {
	apiStats, err := dbc.GetTestResultsStats(ctx, opts)
	if err != nil {
		return nil, err
	}

	results, filteredCount, err := dbModel.FindAndDownloadTestResults(ctx, dbc.env, convertToDBFindAndDownloadTestResultsOptions(opts))
	if db.ResultsNotFound(err) {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    "test results not found",
		}
	} else if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "retrieving test results").Error(),
		}
	}

	if err = apiStats.Import(filteredCount); err != nil {
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

// GetTestResultsFilteredSamples returns test names for the specified test results, filtered by the provided regexes.
func (dbc *DBConnector) GetTestResultsFilteredSamples(ctx context.Context, opts TestSampleOptions) ([]model.APITestResultsSample, error) {
	samples, err := dbModel.GetTestResultsFilteredSamples(ctx, dbc.env, convertToDBFindTestSampleOptions(opts))
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "retrieving test results samples").Error(),
		}
	}

	return importTestResultsSamples(samples)
}

func (dbc *DBConnector) GetFailedTestResultsSample(ctx context.Context, opts TestResultsOptions) ([]string, error) {
	resultDocs, err := dbModel.FindTestResults(ctx, dbc.env, convertToDBFindTestResultsOptions(opts))
	if db.ResultsNotFound(err) {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    "test results not found",
		}
	} else if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "retrieving test results").Error(),
		}
	}

	return extractFailedTestResultsSample(resultDocs...), nil
}

func (dbc *DBConnector) GetTestResultsStats(ctx context.Context, opts TestResultsOptions) (*model.APITestResultsStats, error) {
	stats, err := dbModel.GetTestResultsStats(ctx, dbc.env, convertToDBFindTestResultsOptions(opts))
	if db.ResultsNotFound(err) {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    "test results not found",
		}
	} else if err != nil {
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

///////////////////////////////
// MockConnector Implementation
///////////////////////////////

func (mc *MockConnector) FindTestResults(ctx context.Context, opts TestResultsOptions) (*model.APITestResults, error) {
	return nil, errors.New("not implemented")
}

func (mc *MockConnector) GetTestResultsFilteredSamples(ctx context.Context, opts TestSampleOptions) ([]model.APITestResultsSample, error) {
	return nil, errors.New("not implemented")
}

func (mc *MockConnector) GetFailedTestResultsSample(ctx context.Context, opts TestResultsOptions) ([]string, error) {
	return nil, errors.New("not implemented")
}

func (mc *MockConnector) GetTestResultsStats(ctx context.Context, opts TestResultsOptions) (*model.APITestResultsStats, error) {
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

func convertToDBFindTestSampleOptions(opts TestSampleOptions) dbModel.FindTestSamplesOptions {
	dbOptions := dbModel.FindTestSamplesOptions{TestNameRegexes: opts.TestNameRegexes}
	for _, t := range opts.Tasks {
		dbOptions.Tasks = append(dbOptions.Tasks, dbModel.FindTestResultsOptions{
			TaskID:      t.TaskID,
			DisplayTask: t.DisplayTask,
			Execution:   utility.ToIntPtr(t.Execution),
		})
	}
	return dbOptions
}

func convertToDBFindTestResultsOptions(opts TestResultsOptions) dbModel.FindTestResultsOptions {
	return dbModel.FindTestResultsOptions{
		TaskID:      opts.TaskID,
		Execution:   opts.Execution,
		DisplayTask: opts.DisplayTask,
	}
}

func convertToDBFindAndDownloadTestResultsOptions(opts TestResultsOptions) dbModel.FindAndDownloadTestResultsOptions {
	var filterAndSort *dbModel.FilterAndSortTestResultsOptions
	if opts.FilterAndSort != nil {
		filterAndSort = &dbModel.FilterAndSortTestResultsOptions{
			TestName:     opts.FilterAndSort.TestName,
			Statuses:     opts.FilterAndSort.Statuses,
			GroupID:      opts.FilterAndSort.GroupID,
			SortBy:       dbModel.TestResultsSortBy(opts.FilterAndSort.SortBy),
			SortOrderDSC: opts.FilterAndSort.SortOrderDSC,
			Limit:        opts.FilterAndSort.Limit,
			Page:         opts.FilterAndSort.Page,
		}
		if opts.FilterAndSort.BaseResults != nil {
			baseOpts := convertToDBFindTestResultsOptions(*opts.FilterAndSort.BaseResults)
			filterAndSort.BaseResults = &baseOpts
		}
	}

	return dbModel.FindAndDownloadTestResultsOptions{
		Find:          convertToDBFindTestResultsOptions(opts),
		FilterAndSort: filterAndSort,
	}
}
