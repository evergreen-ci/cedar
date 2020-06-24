package data

import (
	"context"
	"fmt"
	"net/http"

	"github.com/evergreen-ci/cedar/model"
	dataModel "github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/gimlet"
)

/////////////////////////////
// DBConnector Implementation
/////////////////////////////


// FindTestResultsByTaskId queries the database to find all performance
// results with the given taskId and that fall within interval, filtered by
// tags.
func (dbc *DBConnector) FindTestResultsByTaskId(ctx context.Context, taskId string, execution int, emptyExecution bool) ([]dataModel.APITestResult, error) {
	results := model.TestResults{}
	results.Setup(dbc.env)

	options := model.TestResultsFindOptions{
		TaskID: taskId,
		Execution: execution,
		EmptyExecution: emptyExecution,
	}

	if err := results.FindByTaskID(ctx, options); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("database error"),
		}
	}
	if results.IsNil() {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("test results with task_id '%s' not found", taskId),
		}
	}

	it, err := results.Download(ctx); 
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("database error"),
		}
	}

	apiResults := make([]dataModel.APITestResult, 0)
	i := 0
	for it.Next(ctx) {
		
		// i, result := range it {
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