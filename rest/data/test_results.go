package data

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"

	dataModel "github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/gimlet"

	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/pail"
	"github.com/mongodb/anser/db"
	"github.com/pkg/errors"
	"gopkg.in/mgo.v2/bson"
)

/////////////////////////////
// DBConnector Implementation
/////////////////////////////

func (dbc *DBConnector) FindTestResultsByTaskId(ctx context.Context, options dbModel.TestResultsFindOptions) ([]dataModel.APITestResult, error) {
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
			Message:    fmt.Sprintf("failed to download results with task_id %s", options.TaskID),
		}
	}

	apiResults := make([]dataModel.APITestResult, 0)
	i := 0
	for it.Next(ctx) {
		err := apiResults[i].Import(it.Item())
		if err != nil {
			return nil, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    fmt.Sprintf("corrupt data"),
			}
		}
		i++
	}

	return apiResults, nil
}

func (dbc *DBConnector) FindTestResultByTestName(ctx context.Context, opts TestResultsOptions) (*model.APITestResult, error) {
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
			Message:    errors.Wrap(err, "retrieving metadata").Error(),
		}
	}

	bucket, err := testResults.GetBucket(ctx)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "getting bucket").Error(),
		}
	}

	return getAPITestResultFromBucket(ctx, bucket, opts.TestName)
}

///////////////////////////////
// MockConnector Implementation
///////////////////////////////

// FindTestResultsByTaskId queries the mock cache to find all
// test results with the given task id and execution
func (mc *MockConnector) FindTestResultsByTaskId(_ context.Context, options dbModel.TestResultsFindOptions) ([]dataModel.APITestResult, error) {
	results := []dataModel.APITestResult{}
	for _, result := range mc.CachedTestResults {
		if result.Info.TaskID == options.TaskID && result.Info.Execution == options.Execution {
			apiResult := dataModel.APITestResult{}
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
			Message:    fmt.Sprintf("test result with task_id '%s' not found", options.TaskID),
		}
	}
	return results, nil
}

func (mc *MockConnector) FindTestResultByTestName(ctx context.Context, opts TestResultsOptions) (*model.APITestResult, error) {
	var testResults *dbModel.TestResults

	if opts.EmptyExecution {
		var newest *dbModel.TestResults
		for key, _ := range mc.CachedTestResults {
			tr := mc.CachedTestResults[key]
			if tr.Info.TaskID == opts.TaskID && (newest == nil || tr.Info.Execution > newest.Info.Execution) {
				newest = &tr
			}
		}
		testResults = newest
	} else {
		for key, _ := range mc.CachedTestResults {
			tr := mc.CachedTestResults[key]
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
		Prefix: filepath.Join("test_results", testResults.Artifact.Prefix),
	}
	bucket, err := pail.NewLocalBucket(bucketOpts)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "problem creating bucket").Error(),
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
			Message:    errors.Wrap(err, "retrieving test result").Error(),
		}
	}

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
