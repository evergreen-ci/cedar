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

func (dbc *DBConnector) FindTestResultsByTaskId(ctx context.Context, options model.TestResultsFindOptions) ([]dataModel.APITestResult, error) {
	results := model.TestResults{}
	results.Setup(dbc.env)

	if err := results.FindByTaskID(ctx, options); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("failed to find results with task_id %s", options.TaskID),
		}
	}

	it, err := results.Download(ctx); 
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