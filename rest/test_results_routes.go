package rest

import (
	"context"
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

// Run finds and returns the desired test result based on task id.
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

///////////////////////////////////////////////////////////////////////////////
//
// GET /testresults/test_name/{task_id}/{test_name}

type testResultGetByTestNameHandler struct {
	opts data.TestResultsOptions
	sc   data.Connector
}

func makeGetTestResultByTestName(sc data.Connector) gimlet.RouteHandler {
	return &testResultGetByTestNameHandler{
		sc: sc,
	}
}

// Factory returns a pointer to a new testResultGetByTestNameHandler.
func (h *testResultGetByTestNameHandler) Factory() gimlet.RouteHandler {
	return &testResultGetByTestNameHandler{
		sc: h.sc,
	}
}

// Parse fetches the task_id, test_name, and execution (if present)
// from the http request.
func (h *testResultGetByTestNameHandler) Parse(_ context.Context, r *http.Request) error {
	var err error

	h.opts.TaskID = gimlet.GetVars(r)["task_id"]
	h.opts.TestName = gimlet.GetVars(r)["test_name"]
	vals := r.URL.Query()
	if len(vals[execution]) > 0 {
		h.opts.Execution, err = strconv.Atoi(vals[execution][0])
		return err
	} else {
		h.opts.EmptyExecution = true
	}

	return nil
}

// Run finds and returns the desired test result.
func (h *testResultGetByTestNameHandler) Run(ctx context.Context) gimlet.Responder {
	testResult, err := h.sc.FindTestResultByTestName(ctx, h.opts)
	if err != nil {
		err = errors.Wrapf(err, "problem getting test result by task_id '%s' and test_name '%s'", h.opts.TaskID, h.opts.TestName)
		grip.Error(message.WrapError(err, message.Fields{
			"request":   gimlet.GetRequestID(ctx),
			"method":    "GET",
			"route":     "/testresults/test_name/{task_id}/{test_name}",
			"task_id":   h.opts.TaskID,
			"test_name": h.opts.TestName,
		}))
		return gimlet.MakeJSONInternalErrorResponder(err)
	}
	return gimlet.NewJSONResponse(testResult)
}
