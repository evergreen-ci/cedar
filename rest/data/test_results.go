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

// FindTestResultsByTaskId queries the database to find the test
// result with the given id.
// func (dbc *DBConnector) FindTestResultsByTaskId(ctx context.Context, id string) (*dataModel.APIPerformanceResult, error) {
// 	results := model.TestResult{}
// 	results.Setup(dbc.env)
// 	results.ID = id

// 	if err := result.Find(ctx); err != nil {
// 		return nil, gimlet.ErrorResponse{
// 			StatusCode: http.StatusNotFound,
// 			Message:    fmt.Sprintf("test result with id '%s' not found", id),
// 		}
// 	}

// 	apiResult := dataModel.APIPerformanceResult{}
// 	err := apiResult.Import(result)
// 	if err != nil {
// 		return nil, gimlet.ErrorResponse{
// 			StatusCode: http.StatusInternalServerError,
// 			Message:    fmt.Sprintf("corrupt data"),
// 		}
// 	}
// 	return &apiResult, nil
// }


// FindTestResultsByTaskId queries the database to find all performance
// results with the given taskId and that fall within interval, filtered by
// tags.
func (dbc *DBConnector) FindTestResultsByTaskId(ctx context.Context, taskId string, interval model.TimeRange, tags ...string) ([]dataModel.APIPerformanceResult, error) {
	results := model.TestResults{}
	results.Setup(dbc.env)

	options := model.TestResultsFindOptions{
		Interval: interval,
		Info: model.TestResultInfo{
			TaskID: taskId,
			Tags:   tags,
		},
		MaxDepth: 0,
	}

	if err := results.Find(ctx, options); err != nil {
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

	apiResults := make([]dataModel.APITestResult, len(results.Results))
	for i, result := range results.Results {
		err := apiResults[i].Import(result)
		if err != nil {
			return nil, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    fmt.Sprintf("corrupt data"),
			}
		}
	}
	return apiResults, nil
}