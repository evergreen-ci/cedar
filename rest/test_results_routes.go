package rest

import (
	"context"
	"net/http"

	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

///////////////////////////////////////////////////////////////////////////////
//
// GET /testresults/task_id/{task_id}

type testResultsGetByTaskIdHandler struct {
	options model.TestResultsFindOptions
	sc      data.Connector
}

func makeGetTestResultsByTaskId(sc data.Connector) gimlet.RouteHandler {
	return &testResultsGetByTaskIdHandler{
		sc: sc,
	}
}

// Factory returns a pointer to a new testResultsGetByTaskIdHandler.
func (h *testResultsGetByTaskIdHandler) Factory() gimlet.RouteHandler {
	return &testResultsGetByTaskIdHandler{
		sc: h.sc,
	}
}

// Parse fetches the task_id from the http request.
func (h *testResultsGetByTaskIdHandler) Parse(_ context.Context, r *http.Request) error {
	h.options.TaskID = gimlet.GetVars(r)["task_id"]
	return nil
}

// Run calls the data FindTestResultsByTaskId and function returns the
// TestResults from the provider.
func (h *testResultsGetByTaskIdHandler) Run(ctx context.Context) gimlet.Responder {
	options := model.TestResultsFindOptions{
		TaskID:         h.options.TaskID,
		Execution:      h.options.Execution,
		EmptyExecution: h.options.EmptyExecution,
	}

	testResults, err := h.sc.FindTestResultsByTaskId(ctx, options)
	if err != nil {
		err = errors.Wrapf(err, "problem getting test results by task id '%s'", h.options.TaskID)
		grip.Error(message.WrapError(err, message.Fields{
			"request": gimlet.GetRequestID(ctx),
			"method":  "GET",
			"route":   "/testresults/task_id/{task_id}",
			"task_id": h.options.TaskID,
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}
	return gimlet.NewJSONResponse(testResults)
}
