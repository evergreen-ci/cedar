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

	return importTestResults(ctx, results, opts.TestName)
}

func (dbc *DBConnector) GetFailedTestResultsSample(ctx context.Context, opts TestResultsOptions) ([]string, error) {
	resultDocs, err := dbModel.FindTestResults(ctx, dbc.env, convertToDBTestResultsOptions(opts))
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
	stats, err := dbModel.GetTestResultsStats(ctx, dbc.env, convertToDBTestResultsOptions(opts))
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
	apiStats.Import(stats)

	return apiStats, nil
}

///////////////////////////////
// MockConnector Implementation
///////////////////////////////

func (mc *MockConnector) FindTestResults(ctx context.Context, opts TestResultsOptions) ([]model.APITestResult, error) {
	var (
		results []dbModel.TestResult
		err     error
	)

	if opts.DisplayTask {
		results, err = mc.findAndDownloadTestResultsByDisplayTaskID(ctx, opts)
	} else {
		results, err = mc.findAndDownloadTestResultsByTaskID(ctx, opts)
	}
	if err != nil {
		return nil, err
	}

	return importTestResults(ctx, results, opts.TestName)
}

func (mc *MockConnector) GetFailedTestResultsSample(ctx context.Context, opts TestResultsOptions) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    err.Error(),
		}
	}

	if !opts.DisplayTask {
		testResultsDoc, err := mc.findTestResultsByTaskID(ctx, opts)
		if err != nil {
			return nil, err
		}

		return extractFailedTestResultsSample(*testResultsDoc), nil
	}

	testResultsDocs, err := mc.findTestResultsByDisplayTaskID(ctx, opts)
	if err != nil {
		return nil, err
	}
	return extractFailedTestResultsSample(testResultsDocs...), nil
}

func (mc *MockConnector) GetTestResultsStats(ctx context.Context, opts TestResultsOptions) (*model.APITestResultsStats, error) {
	if err := ctx.Err(); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    err.Error(),
		}
	}

	stats := &model.APITestResultsStats{}

	if !opts.DisplayTask {
		testResultsDoc, err := mc.findTestResultsByTaskID(ctx, opts)
		if err != nil {
			return nil, err
		}

		stats.Import(testResultsDoc.Stats)
	} else {
		testResultsDocs, err := mc.findTestResultsByDisplayTaskID(ctx, opts)
		if err != nil {
			return nil, err
		}

		for _, doc := range testResultsDocs {
			stats.TotalCount += doc.Stats.TotalCount
			stats.FailedCount += doc.Stats.FailedCount
		}
	}

	return stats, nil
}

func (mc *MockConnector) findAndDownloadTestResultsByTaskID(ctx context.Context, opts TestResultsOptions) ([]dbModel.TestResult, error) {
	testResultsDoc, err := mc.findTestResultsByTaskID(ctx, opts)
	if err != nil {
		return nil, err
	}

	results, err := testResultsDoc.Download(ctx)
	if err != nil {
		return nil, err
	}

	return results, nil
}

func (mc *MockConnector) findAndDownloadTestResultsByDisplayTaskID(ctx context.Context, opts TestResultsOptions) ([]dbModel.TestResult, error) {
	testResultsDocs, err := mc.findTestResultsByDisplayTaskID(ctx, opts)
	if err != nil {
		return nil, err
	}

	var combinedResults []dbModel.TestResult
	for i := range testResultsDocs {
		results, err := testResultsDocs[i].Download(ctx)
		if err != nil {
			return nil, err
		}
		combinedResults = append(combinedResults, results...)
	}

	return combinedResults, nil
}

func (mc *MockConnector) findTestResultsByTaskID(ctx context.Context, opts TestResultsOptions) (*dbModel.TestResults, error) {
	var testResultsDoc *dbModel.TestResults

	for key, _ := range mc.CachedTestResults {
		tr := mc.CachedTestResults[key]
		if opts.Execution == nil {
			if tr.Info.TaskID == opts.TaskID && (testResultsDoc == nil || tr.Info.Execution > testResultsDoc.Info.Execution) {
				testResultsDoc = &tr
			}
		} else {
			if tr.Info.TaskID == opts.TaskID && tr.Info.Execution == *opts.Execution {
				testResultsDoc = &tr
				break
			}
		}
	}

	if testResultsDoc == nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    "test results not found",
		}
	}

	return testResultsDoc, nil
}

func (mc *MockConnector) findTestResultsByDisplayTaskID(ctx context.Context, opts TestResultsOptions) ([]dbModel.TestResults, error) {
	var (
		testResultsDocs []dbModel.TestResults
		latestExecution int
	)

	for key, _ := range mc.CachedTestResults {
		tr := mc.CachedTestResults[key]
		if tr.Info.DisplayTaskID == opts.TaskID {
			if opts.Execution == nil || tr.Info.Execution == *opts.Execution {
				testResultsDocs = append(testResultsDocs, tr)
			}
		}
		if tr.Info.Execution > latestExecution {
			latestExecution = tr.Info.Execution
		}
	}

	if opts.Execution == nil {
		var filteredDocs []dbModel.TestResults
		for i := range testResultsDocs {
			if testResultsDocs[i].Info.Execution == latestExecution {
				filteredDocs = append(filteredDocs, testResultsDocs[i])
			}
		}
		testResultsDocs = filteredDocs
	}

	if len(testResultsDocs) == 0 {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    "test results not found",
		}
	}

	return testResultsDocs, nil
}

///////////////////
// Helper Functions
///////////////////

func importTestResults(ctx context.Context, results []dbModel.TestResult, testName string) ([]model.APITestResult, error) {
	apiResults := []model.APITestResult{}

	for _, result := range results {
		if testName != "" && testName != result.TestName && testName != result.DisplayTestName {
			continue
		}

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

func convertToDBTestResultsOptions(opts TestResultsOptions) dbModel.TestResultsFindOptions {
	return dbModel.TestResultsFindOptions{
		TaskID:      opts.TaskID,
		Execution:   opts.Execution,
		DisplayTask: opts.DisplayTask,
	}
}
