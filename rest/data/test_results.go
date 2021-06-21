package data

import (
	"context"
	"fmt"
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
	results, err := dbModel.FindAndDownloadTestResults(ctx, dbc.env, convertToDBTestResultsOptions(opts))
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

func (dbc *DBConnector) FindTestResultByTestName(ctx context.Context, opts TestResultsOptions) (*model.APITestResult, error) {
	results, err := dbModel.FindAndDownloadTestResults(ctx, dbc.env, convertToDBTestResultsOptions(opts))
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

	return getAPITestResultByTestName(ctx, results, opts.TestName)
}

///////////////////////////////
// MockConnector Implementation
///////////////////////////////

func (mc *MockConnector) FindTestResults(ctx context.Context, opts TestResultsOptions) ([]model.APITestResult, error) {
	if opts.TaskID != "" {
		return mc.findTestResultsByTaskID(ctx, opts)
	}

	return mc.findTestResultsByDisplayTaskID(ctx, opts)
}

func (mc *MockConnector) findTestResultsByTaskID(ctx context.Context, opts TestResultsOptions) ([]model.APITestResult, error) {
	var testResults *dbModel.TestResults

	for key, _ := range mc.CachedTestResults {
		tr := mc.CachedTestResults[key]
		if opts.EmptyExecution {
			if tr.Info.TaskID == opts.TaskID && (testResults == nil || tr.Info.Execution > testResults.Info.Execution) {
				testResults = &tr
			}
		} else {
			if tr.Info.TaskID == opts.TaskID && tr.Info.Execution == opts.Execution {
				testResults = &tr
				break
			}
		}
	}

	if testResults == nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    "test results not found",
		}
	}

	results, err := testResults.Download(ctx)
	if err != nil {
		return nil, err
	}

	return importTestResults(ctx, results)
}

func (mc *MockConnector) findTestResultsByDisplayTaskID(ctx context.Context, opts TestResultsOptions) ([]model.APITestResult, error) {
	var (
		testResults     []*dbModel.TestResults
		combinedResults []dbModel.TestResult
		latestExecution int
	)

	for key, _ := range mc.CachedTestResults {
		tr := mc.CachedTestResults[key]
		if tr.Info.DisplayTaskID == opts.DisplayTaskID {
			if opts.EmptyExecution || tr.Info.Execution == opts.Execution {
				testResults = append(testResults, &tr)
			}
		}
		if tr.Info.Execution > latestExecution {
			latestExecution = tr.Info.Execution
		}
	}

	if testResults == nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    "test results not found",
		}
	}

	for i := range testResults {
		if !opts.EmptyExecution || testResults[i].Info.Execution == latestExecution {
			results, err := testResults[i].Download(ctx)
			if err != nil {
				return nil, err
			}
			combinedResults = append(combinedResults, results...)
		}
	}

	return importTestResults(ctx, combinedResults)
}

func (mc *MockConnector) FindTestResultByTestName(ctx context.Context, opts TestResultsOptions) (*model.APITestResult, error) {
	var testResults *dbModel.TestResults

	for key, _ := range mc.CachedTestResults {
		tr := mc.CachedTestResults[key]
		if opts.EmptyExecution {
			if tr.Info.TaskID == opts.TaskID && (testResults == nil || tr.Info.Execution > testResults.Info.Execution) {
				testResults = &tr
			}
		} else {
			if tr.Info.TaskID == opts.TaskID && tr.Info.Execution == opts.Execution {
				testResults = &tr
				break
			}
		}
	}

	if testResults == nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    "test results not found",
		}
	}

	results, err := testResults.Download(ctx)
	if err != nil {
		return nil, err
	}

	return getAPITestResultByTestName(ctx, results, opts.TestName)
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

func getAPITestResultByTestName(ctx context.Context, results []dbModel.TestResult, testName string) (*model.APITestResult, error) {
	for _, result := range results {
		if testName == result.TestName {
			apiResult := &model.APITestResult{}
			if err := apiResult.Import(result); err != nil {
				return nil, gimlet.ErrorResponse{
					StatusCode: http.StatusInternalServerError,
					Message:    errors.Wrap(err, "converting test result to output format").Error(),
				}
			}
			return apiResult, ctx.Err()
		}
	}

	return nil, gimlet.ErrorResponse{
		StatusCode: http.StatusNotFound,
		Message:    fmt.Sprintf("test result with test_name '%s' not found", testName),
	}
}

func convertToDBTestResultsOptions(opts TestResultsOptions) dbModel.TestResultsFindOptions {
	return dbModel.TestResultsFindOptions{
		TaskID:         opts.TaskID,
		DisplayTaskID:  opts.DisplayTaskID,
		Execution:      opts.Execution,
		EmptyExecution: opts.EmptyExecution,
	}
}
