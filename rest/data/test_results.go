package data

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/gimlet"
	"github.com/evergreen-ci/pail"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
)

/////////////////////////////
// DBConnector Implementation
/////////////////////////////

func (dbc *DBConnector) FindTestResultsByTaskID(ctx context.Context, opts TestResultsOptions) ([]model.APITestResult, error) {
	it, err := dbModel.FindAndDownloadTestResults(ctx, dbc.env, convertToDBTestResultsOptions(opts))
	if db.ResultsNotFound(err) {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("test results with task_id '%s' not found", opts.TaskID),
		}
	} else if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "retrieving test results with task_id %s", opts.TaskID).Error(),
		}
	}

	return importTestResults(ctx, it)
}

func (dbc *DBConnector) FindTestResultsByDisplayTaskID(ctx context.Context, opts TestResultsOptions) ([]model.APITestResult, error) {
	it, err := dbModel.FindAndDownloadTestResults(ctx, dbc.env, convertToDBTestResultsOptions(opts))
	if db.ResultsNotFound(err) {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("test results with display_task_id '%s' not found", opts.DisplayTaskID),
		}
	} else if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "retrieving test results with display_task_id %s", opts.DisplayTaskID).Error(),
		}
	}

	return importTestResults(ctx, it)
}

func (dbc *DBConnector) FindTestResultByTestName(ctx context.Context, opts TestResultsOptions) (*model.APITestResult, error) {
	results, err := dbModel.FindTestResults(ctx, dbc.env, convertToDBTestResultsOptions(opts))
	if db.ResultsNotFound(err) {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("test results with task_id '%s' not found", opts.TaskID),
		}
	} else if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "retrieving test results for task_id '%s'", opts.TaskID).Error(),
		}
	}

	bucket, err := results[0].GetBucket(ctx)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "getting bucket for task_id '%s'", opts.TaskID).Error(),
		}
	}

	return getAPITestResultFromBucket(ctx, bucket, opts.TestName)
}

///////////////////////////////
// MockConnector Implementation
///////////////////////////////

func (mc *MockConnector) FindTestResultsByTaskID(ctx context.Context, opts TestResultsOptions) ([]model.APITestResult, error) {
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
			Message:    fmt.Sprintf("test results with display_task_id '%s' not found", opts.TaskID),
		}
	}

	bucket, err := getTestResultsBucket(ctx, mc.Bucket, testResults.Artifact.Prefix)
	if err != nil {
		return nil, err
	}

	return importTestResults(ctx, dbModel.NewTestResultsIterator(bucket))
}

func (mc *MockConnector) FindTestResultsByDisplayTaskID(ctx context.Context, opts TestResultsOptions) ([]model.APITestResult, error) {
	var (
		testResults     []*dbModel.TestResults
		its             []dbModel.TestResultsIterator
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

	for _, result := range testResults {
		if !opts.EmptyExecution || result.Info.Execution == latestExecution {
			bucket, err := getTestResultsBucket(ctx, mc.Bucket, result.Artifact.Prefix)
			if err != nil {
				return nil, err
			}
			its = append(its, dbModel.NewTestResultsIterator(bucket))
		}
	}

	if testResults == nil || its == nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("test results with display_task_id '%s' not found", opts.DisplayTaskID),
		}
	}

	return importTestResults(ctx, dbModel.NewMultiTestResultsIterator(its...))
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
			Message:    fmt.Sprintf("test results with task_id '%s' not found", opts.TaskID),
		}
	}

	bucket, err := getTestResultsBucket(ctx, mc.Bucket, testResults.Artifact.Prefix)
	if err != nil {
		return nil, err
	}

	return getAPITestResultFromBucket(ctx, bucket, opts.TestName)
}

///////////////////
// Helper Functions
///////////////////

func importTestResults(ctx context.Context, it dbModel.TestResultsIterator) ([]model.APITestResult, error) {
	apiResults := []model.APITestResult{}

	for it.Next(ctx) {
		apiResult := model.APITestResult{}
		err := apiResult.Import(it.Item())
		if err != nil {
			return nil, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    errors.Wrapf(err, "importing result into APITestResult struct").Error(),
			}
		}
		apiResults = append(apiResults, apiResult)
	}
	if err := it.Err(); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "iterating through test results").Error(),
		}
	}

	return apiResults, nil
}

func getAPITestResultFromBucket(ctx context.Context, bucket pail.Bucket, testName string) (*model.APITestResult, error) {
	tr, err := bucket.Get(ctx, testName)
	if pail.IsKeyNotFoundError(err) {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("test result with test_name '%s' not found", testName),
		}
	} else if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "retrieving test result with test_name '%s'", testName).Error(),
		}
	}
	defer func() {
		grip.Warning(errors.Wrap(tr.Close(), "closing file"))
	}()

	data, err := ioutil.ReadAll(tr)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "reading data").Error(),
		}
	}

	var result dbModel.TestResult
	if err := bson.Unmarshal(data, &result); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "unmarshalling test result").Error(),
		}
	}

	apiResult := &model.APITestResult{}
	if err := apiResult.Import(result); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "converting test result to output format").Error(),
		}
	}

	return apiResult, ctx.Err()
}

func getTestResultsBucket(ctx context.Context, path, prefix string) (pail.Bucket, error) {
	bucketOpts := pail.LocalOptions{
		Path:   path,
		Prefix: prefix,
	}
	bucket, err := pail.NewLocalBucket(bucketOpts)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "creating bucket").Error(),
		}
	}

	return bucket, nil
}

func convertToDBTestResultsOptions(opts TestResultsOptions) dbModel.TestResultsFindOptions {
	return dbModel.TestResultsFindOptions{
		TaskID:         opts.TaskID,
		DisplayTaskID:  opts.DisplayTaskID,
		Execution:      opts.Execution,
		EmptyExecution: opts.EmptyExecution,
	}
}
