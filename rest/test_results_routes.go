package rest

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/gimlet"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

type testResultsBaseHandler struct {
	sc      data.Connector
	payload struct {
		TaskOpts   []data.TestResultsTaskOptions         `json:"tasks"`
		FilterOpts *data.TestResultsFilterAndSortOptions `json:"filter"`
	}
}

func (h *testResultsBaseHandler) Parse(_ context.Context, r *http.Request) error {
	if r.Body == nil {
		return errors.New("missing request payload")
	}
	body := utility.NewRequestReader(r)
	defer body.Close()

	if err := json.NewDecoder(r.Body).Decode(&h.payload); err != nil {
		return errors.Wrap(err, "decoding JSON request payload")
	}

	if len(h.payload.TaskOpts) == 0 {
		return errors.New("must specify at least one task in the request payload")
	}

	return nil
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /test_results/tasks

type testResultsGetByTasksHandler struct {
	testResultsBaseHandler
}

func makeGetTestResultsByTasks(sc data.Connector) *testResultsGetByTasksHandler {
	h := &testResultsGetByTasksHandler{}
	h.sc = sc

	return h
}

func (h *testResultsGetByTasksHandler) Factory() gimlet.RouteHandler {
	newHandler := &testResultsGetByTasksHandler{}
	newHandler.sc = h.sc

	return newHandler
}

func (h *testResultsGetByTasksHandler) Run(ctx context.Context) gimlet.Responder {
	testResults, err := h.sc.FindTestResults(ctx, h.payload.TaskOpts, h.payload.FilterOpts)
	if err != nil {
		err = errors.Wrap(err, "getting test results by tasks")
		logFindError(err, message.Fields{
			"request":         gimlet.GetRequestID(ctx),
			"method":          "GET",
			"route":           "/test_results/tasks",
			"request_payload": h.payload,
		})
		return gimlet.MakeJSONErrorResponder(err)
	}

	return gimlet.NewJSONResponse(testResults)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /test_results/tasks/stats

type testResultsStatsGetByTasksHandler struct {
	testResultsBaseHandler
}

func makeGetTestResultsStatsByTasks(sc data.Connector) *testResultsStatsGetByTasksHandler {
	h := &testResultsStatsGetByTasksHandler{}
	h.sc = sc

	return h
}

func (h *testResultsStatsGetByTasksHandler) Factory() gimlet.RouteHandler {
	newHandler := &testResultsStatsGetByTasksHandler{}
	newHandler.sc = h.sc

	return newHandler
}

func (h *testResultsStatsGetByTasksHandler) Run(ctx context.Context) gimlet.Responder {
	stats, err := h.sc.FindTestResultsStats(ctx, h.payload.TaskOpts)
	if err != nil {
		err = errors.Wrap(err, "getting test results stats by tasks")
		logFindError(err, message.Fields{
			"request":         gimlet.GetRequestID(ctx),
			"method":          "GET",
			"route":           "/test_results/tasks/stats",
			"request_payload": h.payload,
		})
		return gimlet.MakeJSONErrorResponder(err)
	}

	return gimlet.NewJSONResponse(stats)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /test_results/tasks/failed_sample

type testResultsFailedSampleGetByTasksHandler struct {
	testResultsBaseHandler
}

func makeGetTestResultsFailedSampleByTasks(sc data.Connector) *testResultsFailedSampleGetByTasksHandler {
	h := &testResultsFailedSampleGetByTasksHandler{}
	h.sc = sc

	return h
}

func (h *testResultsFailedSampleGetByTasksHandler) Factory() gimlet.RouteHandler {
	newHandler := &testResultsFailedSampleGetByTasksHandler{}
	newHandler.sc = h.sc

	return newHandler
}

func (h *testResultsFailedSampleGetByTasksHandler) Run(ctx context.Context) gimlet.Responder {
	sample, err := h.sc.FindFailedTestResultsSample(ctx, h.payload.TaskOpts)
	if err != nil {
		err = errors.Wrap(err, "getting failed test results sample by tasks")
		logFindError(err, message.Fields{
			"request":         gimlet.GetRequestID(ctx),
			"method":          "GET",
			"route":           "/test_results/tasks/failed_sample",
			"request_payload": h.payload,
		})
		return gimlet.MakeJSONErrorResponder(err)
	}

	return gimlet.NewJSONResponse(sample)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /test_results/filtered_samples

type testResultsGetFilteredSamplesHandler struct {
	sc      data.Connector
	payload struct {
		Tasks        []data.TestResultsTaskOptions `json:"tasks"`
		RegexFilters []string                      `json:"regex_filters"`
	}
}

func makeGetTestResultsFilteredSamples(sc data.Connector) gimlet.RouteHandler {
	return &testResultsGetFilteredSamplesHandler{
		sc: sc,
	}
}

// Factory returns a pointer to a new testResultsGetFilteredSamplesHandler.
func (h *testResultsGetFilteredSamplesHandler) Factory() gimlet.RouteHandler {
	return &testResultsGetFilteredSamplesHandler{
		sc: h.sc,
	}
}

func (h *testResultsGetFilteredSamplesHandler) Parse(_ context.Context, r *http.Request) error {
	body := utility.NewRequestReader(r)
	defer body.Close()

	if err := utility.ReadJSON(body, &h.payload); err != nil {
		return errors.Wrap(err, "argument read error")
	}

	return nil
}

// Run finds and returns the filtered test results sample.
func (h *testResultsGetFilteredSamplesHandler) Run(ctx context.Context) gimlet.Responder {
	samples, err := h.sc.FindFailedTestResultsSamples(ctx, h.payload.Tasks, h.payload.RegexFilters)
	if err != nil {
		err = errors.Wrap(err, "getting filtered test results sample")
		grip.Error(message.WrapError(err, message.Fields{
			"request": gimlet.GetRequestID(ctx),
			"method":  "GET",
			"route":   "/test_results/filtered_samples",
		}))
		return gimlet.MakeJSONInternalErrorResponder(err)
	}

	return gimlet.NewJSONResponse(samples)
}
