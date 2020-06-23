package rest

import (
	"context"
	"net/http"

	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/gimlet"
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

type testResultsGetByTestNameHandler struct {
}

func makeGetTestResultsByTestName(sc data.Connector) gimlet.RouteHandler {
	return &testResultsGetByTestNameHandler{
		sc: sc,
	}
}

// Factory
func (h *testResultsGetByTestNameHandler) Factory() gimlet.RouteHandler {
}

// Parse
func (h *testResultsGetByTestNameHandler) Parse(_ context.Context, r *http.Request) error {

}

// Run
func (h *testResultsGetByTestNameHandler) Run(ctx context.Context) gimlet.Responder {

}
