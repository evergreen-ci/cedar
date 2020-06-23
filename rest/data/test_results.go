package data

import (
	"context"

	"github.com/evergreen-ci/cedar/rest/model"
)

/////////////////////////////
// DBConnector Implementation
/////////////////////////////

func (dbc *DBConnector) FindTestResultsByTestName(ctx context.Context, opts TestResultsOptions) (model.APITestResult, error) {

	// find by taskid, execution

	// get bucket

	// get single by key = test name from bucket
	// return not found error if not

	// download item (llok at test_results_iterator)

	// return
	return model.APITestResult{}, nil
}

///////////////////////////////
// MockConnector Implementation
///////////////////////////////

func (dbc *MockConnector) FindTestResultsByTestName(ctx context.Context, opts TestResultsOptions) (model.APITestResult, error) {

	// find by taskid, execution

	// get bucket

	// get single by key = test name from bucket
	// return not found error if not

	// download item (llok at test_results_iterator)

	// return
	return model.APITestResult{}, nil
}
