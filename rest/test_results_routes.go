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

///////////////////////////////////////////////////////////////////////////////
//
// GET /testresults/task_id/{task_id}

// Factory

// Parse

// Run

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

// Factory returns a pointer to a new testResultGetByTestNameHandler
func (h *testResultGetByTestNameHandler) Factory() gimlet.RouteHandler {
	return &testResultGetByTestNameHandler{
		sc: h.sc,
	}
}

// Parse fetches the task_id, test_name, and execution (if present)
// from the http request.
func (h *testResultGetByTestNameHandler) Parse(_ context.Context, r *http.Request) error {
	catcher := grip.NewBasicCatcher()
	var err error

	h.opts.TaskID = gimlet.GetVars(r)["task_id"]
	h.opts.TestName = gimlet.GetVars(r)["test_name"]
	vals := r.URL.Query()
	if len(vals[execution]) > 0 {
		h.opts.Execution, err = strconv.Atoi(vals[execution][0])
		h.opts.EmptyExecution = false
		catcher.Add(err)
	} else {
		h.opts.EmptyExecution = true
	}

	return catcher.Resolve()
}

// Run calls FindFindTestResultByTestName and returns the test result.
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
		return gimlet.MakeJSONErrorResponder(err)
	}
	return gimlet.NewJSONResponse(testResult)
}
