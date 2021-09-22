package data

import (
	"context"
	"net/http"

	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/anser/db"
	"github.com/pkg/errors"
)

/////////////////////////////
// DBConnector Implementation
/////////////////////////////

func (dbc *DBConnector) FindTestResults(ctx context.Context, opts TestResultsOptions) ([]model.APITestResult, error) {
	results, _, err := dbModel.FindAndDownloadTestResults(ctx, dbc.env, convertToDBFindAndDownloadTestResultsOptions(opts))
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

	return importTestResults(ctx, results)
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

func (mc *MockConnector) FindTestResults(ctx context.Context, opts TestResultsOptions) ([]model.APITestResult, error) {
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

func extractFailedTestResultsSample(results ...dbModel.TestResults) []string {
	var sample []string
	for i := 0; i < len(results) && len(sample) < dbModel.FailedTestsSampleSize; i++ {
		sample = append(sample, results[i].FailedTestsSample...)
	}

	return sample
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
