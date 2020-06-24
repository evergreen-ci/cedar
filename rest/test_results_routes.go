package rest

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

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
	taskId   		string
	execution		int
	emptyExecution 	bool
	sc	       		data.Connector
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
	h.taskId = gimlet.GetVars(r)["task_id"]
	vals := r.URL.Query()
	var err error
	return err
}

// Run calls the data FindTestResultsByTaskId and function returns the
// TestResults from the provider.
func (h *testResultsGetByTaskIdHandler) Run(ctx context.Context) gimlet.Responder {
	options := model.TestResultsFindOptions{
		TaskID: h.taskId,
		Execution: h.execution,
		EmptyExecution: h.emptyExecution,
	}
	
	testResults, err := h.sc.FindTestResultsByTaskId(ctx, options)
	if err != nil {
		err = errors.Wrapf(err, "problem getting test results by task id '%s'", h.taskId)
		grip.Error(message.WrapError(err, message.Fields{
			"request": gimlet.GetRequestID(ctx),
			"method":  "GET",
			"route":   "/testresults/task_id/{task_id}",
			"task_id": h.taskId,
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}
	return gimlet.NewJSONResponse(perfResults)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /testresults/test_name/{task_id}/{test_name}

// Factory

// Parse

// Run
