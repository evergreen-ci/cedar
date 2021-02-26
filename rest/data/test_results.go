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
	it, err := dbModel.FindAndDownloadTestResults(ctx, dbc.env, convertToDBOpts(opts))
	if db.ResultsNotFound(err) {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("test results with task id '%s' not found", opts.TaskID),
		}
	} else if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "retrieving test results with task_id %s", opts.TaskID).Error(),
		}
	}

	apiResults := make([]model.APITestResult, 0)
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

func (dbc *DBConnector) FindTestResultByTestName(ctx context.Context, opts TestResultsOptions) (*model.APITestResult, error) {
	results, err := dbModel.FindTestResults(ctx, dbc.env, convertToDBOpts(opts))
	if db.ResultsNotFound(err) {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("test results with task id '%s' not found", opts.TaskID),
		}
	} else if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "retrieving test results for task id '%s'", opts.TaskID).Error(),
		}
	}

	bucket, err := results[0].GetBucket(ctx)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "bucket for task id '%s'", opts.TaskID).Error(),
		}
	}

	return getAPITestResultFromBucket(ctx, bucket, opts.TestName)
}

///////////////////////////////
// MockConnector Implementation
///////////////////////////////

func (mc *MockConnector) FindTestResultsByTaskID(ctx context.Context, opts TestResultsOptions) ([]model.APITestResult, error) {
	apiResults := []model.APITestResult{}
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
			Message:    fmt.Sprintf("test results with task id '%s' not found", opts.TaskID),
		}
	}

	bucketOpts := pail.LocalOptions{
		Path:   mc.Bucket,
		Prefix: testResults.Artifact.Prefix,
	}
	bucket, err := pail.NewLocalBucket(bucketOpts)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "creating bucket").Error(),
		}
	}
	it := dbModel.NewTestResultsIterator(bucket)

	for it.Next(ctx) {
		result := it.Item()

		apiResult := model.APITestResult{}
		err := apiResult.Import(result)
		if err != nil {
			return nil, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    fmt.Sprintf("corrupt data from MockConnector"),
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

	if len(apiResults) == 0 {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("test results with task_id '%s' not found", opts.TaskID),
		}
	}
	return apiResults, nil
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
			Message:    fmt.Sprintf("test results with task id '%s' not found", opts.TaskID),
		}
	}

	bucketOpts := pail.LocalOptions{
		Path:   mc.Bucket,
		Prefix: testResults.Artifact.Prefix,
	}
	bucket, err := pail.NewLocalBucket(bucketOpts)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "creating bucket for task id '%s'", opts.TaskID).Error(),
		}
	}

	return getAPITestResultFromBucket(ctx, bucket, opts.TestName)
}

func getAPITestResultFromBucket(ctx context.Context, bucket pail.Bucket, testName string) (*model.APITestResult, error) {
	tr, err := bucket.Get(ctx, testName)
	if pail.IsKeyNotFoundError(err) {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("test result with test name '%s' not found", testName),
		}
	} else if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "retrieving test result with test name '%s'", testName).Error(),
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

func convertToDBOpts(opts TestResultsOptions) dbModel.TestResultsFindOptions {
	return dbModel.TestResultsFindOptions{
		TaskID:         opts.TaskID,
		DisplayTaskID:  opts.DisplayTaskID,
		Execution:      opts.Execution,
		EmptyExecution: opts.EmptyExecution,
	}
}
