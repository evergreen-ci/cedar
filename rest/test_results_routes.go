package rest

import (
	"context"
	"net/http"
	"strconv"

	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

const (
	isDisplayTask = "display_task"
	testName      = "test_name"
)

type testResultsBaseHandler struct {
	opts data.TestResultsOptions
}

// Parse fetches the task_id from the http request.
func (h *testResultsBaseHandler) Parse(_ context.Context, r *http.Request) error {
	h.opts.TaskID = gimlet.GetVars(r)["task_id"]

	vals := r.URL.Query()
	h.opts.TestName = vals.Get(testName)
	if len(vals[execution]) > 0 {
		var err error
		h.opts.Execution, err = strconv.Atoi(vals[execution][0])
		return err
	} else {
		h.opts.EmptyExecution = true
	}
	if vals.Get(isDisplayTask) == trueString {
		h.opts.DisplayTask = true
	}

	return nil
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /test_results/task_id/{task_id}

type testResultsGetByTaskIDHandler struct {
	sc data.Connector
	testResultsBaseHandler
}

func makeGetTestResultsByTaskID(sc data.Connector) gimlet.RouteHandler {
	return &testResultsGetByTaskIDHandler{
		sc: sc,
	}
}

// Factory returns a pointer to a new testResultsGetByTaskIDHandler.
func (h *testResultsGetByTaskIDHandler) Factory() gimlet.RouteHandler {
	return &testResultsGetByTaskIDHandler{
		sc: h.sc,
	}
}

// Run finds and returns the desired test result based on task_id.
func (h *testResultsGetByTaskIDHandler) Run(ctx context.Context) gimlet.Responder {
	testResults, err := h.sc.FindTestResults(ctx, h.opts)
	if err != nil {
		err = errors.Wrapf(err, "problem getting test results by task_id '%s'", h.opts.TaskID)
		grip.Error(message.WrapError(err, message.Fields{
			"request":    gimlet.GetRequestID(ctx),
			"method":     "GET",
			"route":      "/testresults/task_id/{task_id}",
			"task_id":    h.opts.TaskID,
			"test_name":  h.opts.TestName,
			"is_display": h.opts.DisplayTask,
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}
	return gimlet.NewJSONResponse(testResults)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /test_results/task_id/{task_id}/failed_sample

type testResultsGetFailedSampleHandler struct {
	sc data.Connector
	testResultsBaseHandler
}

func makeGetTestResultsFailedSample(sc data.Connector) gimlet.RouteHandler {
	return &testResultsGetFailedSampleHandler{
		sc: sc,
	}
}

// Factory returns a pointer to a new testResultsGetFailedSampleHandler.
func (h *testResultsGetFailedSampleHandler) Factory() gimlet.RouteHandler {
	return &testResultsGetFailedSampleHandler{
		sc: h.sc,
	}
}

// Run finds and returns the desired failed test results sample.
func (h *testResultsGetFailedSampleHandler) Run(ctx context.Context) gimlet.Responder {
	sample, err := h.sc.FindFailedTestResultsSample(ctx, h.opts)
	if err != nil {
		err = errors.Wrapf(err, "problem getting test result by task_id '%s'", h.opts.TaskID)
		grip.Error(message.WrapError(err, message.Fields{
			"request":         gimlet.GetRequestID(ctx),
			"method":          "GET",
			"route":           "/testresults/task_id/{task_id}/failed_sample",
			"task_id":         h.opts.TaskID,
			"is_display_task": h.opts.DisplayTask,
		}))
		return gimlet.MakeJSONInternalErrorResponder(err)
	}

	return gimlet.NewJSONResponse(sample)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /test_results/display_task_id/{display_task_id}

type testResultsGetByDisplayTaskIDHandler struct {
	opts data.TestResultsOptions
	sc   data.Connector
}

func makeGetTestResultsByDisplayTaskID(sc data.Connector) gimlet.RouteHandler {
	return &testResultsGetByDisplayTaskIDHandler{
		sc: sc,
	}
}

// Factory returns a pointer to a new testResultsGetByDisplayTaskIDHandler.
func (h *testResultsGetByDisplayTaskIDHandler) Factory() gimlet.RouteHandler {
	return &testResultsGetByDisplayTaskIDHandler{
		sc: h.sc,
	}
}

// Parse fetches the display_task_id from the http request.
func (h *testResultsGetByDisplayTaskIDHandler) Parse(_ context.Context, r *http.Request) error {
	var err error

	h.opts.TaskID = gimlet.GetVars(r)["display_task_id"]
	h.opts.DisplayTask = true

	vals := r.URL.Query()
	if len(vals[execution]) > 0 {
		h.opts.Execution, err = strconv.Atoi(vals[execution][0])
		return err
	}
	h.opts.EmptyExecution = true

	return nil
}

// Run finds and returns the desired test result based on the display_task_id.
func (h *testResultsGetByDisplayTaskIDHandler) Run(ctx context.Context) gimlet.Responder {
	testResults, err := h.sc.FindTestResults(ctx, h.opts)
	if err != nil {
		err = errors.Wrapf(err, "problem getting test results by display_task_id '%s'", h.opts.TaskID)
		grip.Error(message.WrapError(err, message.Fields{
			"request": gimlet.GetRequestID(ctx),
			"method":  "GET",
			"route":   "/testresults/display_task_id/{display_task_id}",
			"task_id": h.opts.TaskID,
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}
	return gimlet.NewJSONResponse(testResults)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /test_results/test_name/{task_id}/{test_name}

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
	results, err := h.sc.FindTestResults(ctx, h.opts)
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

	if len(results) == 0 {
		return gimlet.MakeJSONErrorResponder(gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    "test result not found",
		})
	}
	return gimlet.NewJSONResponse(&results[0])
}
