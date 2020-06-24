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

const (
	testResultsStartAt = "started_after"
	testResultsEndAt   = "finished_before"
)

///////////////////////////////////////////////////////////////////////////////
//
// GET /testresults/task_id/{task_id}

type testResultsGetByTaskIdHandler struct {
	taskId   string
	interval model.TimeRange
	tags     []string
	sc       data.Connector
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
	h.tags = vals["tags"]
	var err error
	h.interval, err = parseTimeRange(vals, testResultsStartAt, testResultsEndAt)
	return err
}

// Run calls the data FindTestResultsByTaskId and function returns the
// TestResults from the provider.
func (h *testResultsGetByTaskIdHandler) Run(ctx context.Context) gimlet.Responder {
	perfResults, err := h.sc.FindTestResultsByTaskId(ctx, h.taskId, h.interval, h.tags...)
	if err != nil {
		err = errors.Wrapf(err, "problem getting test results by task id '%s'", h.taskId)
		grip.Error(message.WrapError(err, message.Fields{
			"request": gimlet.GetRequestID(ctx),
			"method":  "GET",
			"route":   "/testResults/task_id/{task_id}",
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
