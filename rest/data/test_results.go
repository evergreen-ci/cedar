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
	"gopkg.in/mgo.v2/bson"
)

/////////////////////////////
// DBConnector Implementation
/////////////////////////////

func (dbc *DBConnector) FindTestResultsByTaskId(ctx context.Context, options dbModel.TestResultsFindOptions) ([]model.APITestResult, error) {
	results := dbModel.TestResults{}
	results.Setup(dbc.env)

	if err := results.FindByTaskID(ctx, options); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to find results with task_id %s", options.TaskID),
		}
	}

	it, err := results.Download(ctx)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "failed to download results with task_id %s", options.TaskID).Error(),
		}
	}

	apiResults := make([]model.APITestResult, 0)
	for it.Next(ctx) {
		apiResult := model.APITestResult{}
		err := apiResult.Import(it.Item())
		if err != nil {
			return nil, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    errors.Wrapf(err, "failed to import result into APITestResult struct").Error(),
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

func (dbc *DBConnector) FindTestResultByTestName(ctx context.Context, opts TestResultsTestNameOptions) (*model.APITestResult, error) {
	dbOpts := dbModel.TestResultsFindOptions{
		TaskID:         opts.TaskID,
		Execution:      opts.Execution,
		EmptyExecution: opts.EmptyExecution,
	}

	testResults := &dbModel.TestResults{}
	testResults.Setup(dbc.env)
	if err := testResults.FindByTaskID(ctx, dbOpts); db.ResultsNotFound(err) {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("tests with task id '%s' not found", opts.TaskID),
		}
	} else if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "problem retrieving metadata for task id '%s'", opts.TaskID).Error(),
		}
	}

	bucket, err := testResults.GetBucket(ctx)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "problem creating bucket for task id '%s'", opts.TaskID).Error(),
		}
	}

	return getAPITestResultFromBucket(ctx, bucket, opts.TestName)
}

///////////////////////////////
// MockConnector Implementation
///////////////////////////////

func (mc *MockConnector) FindTestResultsByTaskId(ctx context.Context, opts dbModel.TestResultsFindOptions) ([]model.APITestResult, error) {
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
			Message:    fmt.Sprintf("tests with task id '%s' not found", opts.TaskID),
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
			Message:    errors.Wrapf(err, "problem creating bucket").Error(),
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
			Message:    fmt.Sprintf("Mock Connector test result with task_id '%s' not found", opts.TaskID),
		}
	}
	return apiResults, nil
}

func (mc *MockConnector) FindTestResultByTestName(ctx context.Context, opts TestResultsTestNameOptions) (*model.APITestResult, error) {
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
			Message:    fmt.Sprintf("tests with task id '%s' not found", opts.TaskID),
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
			Message:    errors.Wrapf(err, "problem creating bucket for task id '%s'", opts.TaskID).Error(),
		}
	}

	return getAPITestResultFromBucket(ctx, bucket, opts.TestName)
}

func getAPITestResultFromBucket(ctx context.Context, bucket pail.Bucket, testName string) (*model.APITestResult, error) {
	tr, err := bucket.Get(ctx, testName)
	if pail.IsKeyNotFoundError(err) {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("test with specified task id and test name not found"),
		}
	} else if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "problem retrieving test result").Error(),
		}
	}
	defer func() {
		grip.Warning(errors.Wrap(tr.Close(), "problem closing file"))
	}()

	data, err := ioutil.ReadAll(tr)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "problem reading data").Error(),
		}
	}

	var result dbModel.TestResult
	if err := bson.Unmarshal(data, &result); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "problem unmarshalling test result").Error(),
		}
	}

	apiResult := &model.APITestResult{}
	if err := apiResult.Import(result); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "problem converting test result to output format").Error(),
		}
	}

	return apiResult, ctx.Err()
}
