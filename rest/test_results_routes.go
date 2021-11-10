package rest

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/gimlet"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

const (
	isDisplayTask         = "display_task"
	testResultsTestName   = "test_name"
	testResultsStatus     = "status"
	testResultsGroupID    = "group_id"
	testResultsSortBy     = "sort_by"
	testResultsSortDSC    = "sort_order_dsc"
	testResultsLimit      = "limit"
	testResultsPage       = "page"
	testResultsBaseTaskID = "base_task_id"
)

type testResultsBaseHandler struct {
	opts data.TestResultsOptions
}

// Parse fetches the task_id from the http request.
func (h *testResultsBaseHandler) Parse(_ context.Context, r *http.Request) error {
	h.opts.TaskID = gimlet.GetVars(r)["task_id"]

	vals := r.URL.Query()
	if len(vals[execution]) > 0 {
		exec, err := strconv.Atoi(vals[execution][0])
		if err != nil {
			return err
		}
		h.opts.Execution = utility.ToIntPtr(exec)
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

// Parse fetches the task_id from the http request and any filter, sort, or
// pagination options.
func (h *testResultsGetByTaskIDHandler) Parse(ctx context.Context, r *http.Request) error {
	catcher := grip.NewBasicCatcher()

	catcher.Add(h.testResultsBaseHandler.Parse(ctx, r))
	vals := r.URL.Query()
	testName := vals.Get(testResultsTestName)
	statuses := vals[testResultsStatus]
	groupID := vals.Get(testResultsGroupID)
	sortBy := vals.Get(testResultsSortBy)
	baseTaskID := vals.Get(testResultsBaseTaskID)
	var limit, page int
	if len(vals[testResultsLimit]) > 0 {
		var err error
		limit, err = strconv.Atoi(vals[testResultsLimit][0])
		catcher.Add(err)
	}
	if len(vals[testResultsPage]) > 0 {
		var err error
		page, err = strconv.Atoi(vals[testResultsPage][0])
		catcher.Add(err)
	}

	if testName == "" && len(statuses) == 0 && groupID == "" && sortBy == "" && baseTaskID == "" && limit <= 0 && page <= 0 {
		return catcher.Resolve()
	}

	h.opts.FilterAndSort = &data.TestResultsFilterAndSortOptions{
		TestName: testName,
		Statuses: statuses,
		GroupID:  groupID,
		SortBy:   sortBy,
		Limit:    limit,
		Page:     page,
	}
	if vals.Get(testResultsSortDSC) == trueString {
		h.opts.FilterAndSort.SortOrderDSC = true
	}
	if baseTaskID != "" {
		h.opts.FilterAndSort.BaseResults = &data.TestResultsOptions{
			TaskID:      baseTaskID,
			DisplayTask: h.opts.DisplayTask,
		}
	}

	return catcher.Resolve()
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
		err = errors.Wrapf(err, "getting test results by task_id '%s'", h.opts.TaskID)
		grip.Error(message.WrapError(err, message.Fields{
			"request":    gimlet.GetRequestID(ctx),
			"method":     "GET",
			"route":      "/test_results/task_id/{task_id}",
			"task_id":    h.opts.TaskID,
			"is_display": h.opts.DisplayTask,
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}

	var resp gimlet.Responder
	resp = gimlet.NewJSONResponse(testResults)
	if h.opts.FilterAndSort != nil && h.opts.FilterAndSort.Limit > 0 {
		pages := &gimlet.ResponsePages{
			Prev: &gimlet.Page{
				BaseURL:         h.sc.GetBaseURL(),
				KeyQueryParam:   testResultsPage,
				LimitQueryParam: testResultsLimit,
				Key:             fmt.Sprintf("%d", h.opts.FilterAndSort.Page),
				Limit:           h.opts.FilterAndSort.Limit,
				Relation:        "prev",
			},
		}
		if len(testResults.Results) > 0 {
			pages.Next = &gimlet.Page{
				BaseURL:         h.sc.GetBaseURL(),
				KeyQueryParam:   testResultsPage,
				LimitQueryParam: testResultsLimit,
				Key:             fmt.Sprintf("%d", h.opts.FilterAndSort.Page+1),
				Limit:           h.opts.FilterAndSort.Limit,
				Relation:        "next",
			}
		}

		if err := resp.SetPages(pages); err != nil {
			return gimlet.MakeJSONErrorResponder(errors.Wrap(err, "setting response pages"))
		}
	}

	return resp
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /test_results/filtered_sample

type testResultsGetFilteredSamplesHandler struct {
	sc      data.Connector
	options []data.TestSampleOptions
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

	if err := utility.ReadJSON(body, h.options); err != nil {
		return errors.Wrap(err, "Argument read error")
	}

	return nil
}

// Run finds and returns the filtered test results sample.
func (h *testResultsGetFilteredSamplesHandler) Run(ctx context.Context) gimlet.Responder {
	samples, err := h.sc.GetTestResultsFilteredSamples(ctx, h.options)
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
	sample, err := h.sc.GetFailedTestResultsSample(ctx, h.opts)
	if err != nil {
		err = errors.Wrapf(err, "getting failed test results sample by task_id '%s'", h.opts.TaskID)
		grip.Error(message.WrapError(err, message.Fields{
			"request":         gimlet.GetRequestID(ctx),
			"method":          "GET",
			"route":           "/test_results/task_id/{task_id}/failed_sample",
			"task_id":         h.opts.TaskID,
			"is_display_task": h.opts.DisplayTask,
		}))
		return gimlet.MakeJSONInternalErrorResponder(err)
	}

	return gimlet.NewJSONResponse(sample)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /test_results/task_id/{task_id}/stats

type testResultsGetStatsHandler struct {
	sc data.Connector
	testResultsBaseHandler
}

func makeGetTestResultsStats(sc data.Connector) gimlet.RouteHandler {
	return &testResultsGetStatsHandler{
		sc: sc,
	}
}

// Factory returns a pointer to a new testResultsGetStatsHandler.
func (h *testResultsGetStatsHandler) Factory() gimlet.RouteHandler {
	return &testResultsGetStatsHandler{
		sc: h.sc,
	}
}

// Run finds and returns the desired failed test results stats.
func (h *testResultsGetStatsHandler) Run(ctx context.Context) gimlet.Responder {
	stats, err := h.sc.GetTestResultsStats(ctx, h.opts)
	if err != nil {
		err = errors.Wrapf(err, "getting test results stats by task_id '%s'", h.opts.TaskID)
		grip.Error(message.WrapError(err, message.Fields{
			"request":         gimlet.GetRequestID(ctx),
			"method":          "GET",
			"route":           "/test_results/task_id/{task_id}/stats",
			"task_id":         h.opts.TaskID,
			"is_display_task": h.opts.DisplayTask,
		}))
		return gimlet.MakeJSONInternalErrorResponder(err)
	}

	return gimlet.NewJSONResponse(stats)
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

	h.opts.TaskID = gimlet.GetVars(r)["display_task_id"]
	h.opts.DisplayTask = true

	vals := r.URL.Query()
	if len(vals[execution]) > 0 {
		exec, err := strconv.Atoi(vals[execution][0])
		if err != nil {
			return err
		}
		h.opts.Execution = utility.ToIntPtr(exec)
	}

	return nil
}

// Run finds and returns the desired test result based on the display_task_id.
func (h *testResultsGetByDisplayTaskIDHandler) Run(ctx context.Context) gimlet.Responder {
	testResults, err := h.sc.FindTestResults(ctx, h.opts)
	if err != nil {
		err = errors.Wrapf(err, "getting test results by display_task_id '%s'", h.opts.TaskID)
		grip.Error(message.WrapError(err, message.Fields{
			"request": gimlet.GetRequestID(ctx),
			"method":  "GET",
			"route":   "/test_results/display_task_id/{display_task_id}",
			"task_id": h.opts.TaskID,
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}
	return gimlet.NewJSONResponse(testResults.Results)
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
	h.opts.TaskID = gimlet.GetVars(r)["task_id"]
	h.opts.FilterAndSort = &data.TestResultsFilterAndSortOptions{
		TestName: gimlet.GetVars(r)["test_name"],
	}
	vals := r.URL.Query()
	if len(vals[execution]) > 0 {
		exec, err := strconv.Atoi(vals[execution][0])
		if err != nil {
			return err
		}
		h.opts.Execution = utility.ToIntPtr(exec)
	}

	return nil
}

// Run finds and returns the desired test result.
func (h *testResultGetByTestNameHandler) Run(ctx context.Context) gimlet.Responder {
	testResults, err := h.sc.FindTestResults(ctx, h.opts)
	if err != nil {
		err = errors.Wrapf(err, "getting test result by task_id '%s' and test_name '%s'", h.opts.TaskID, h.opts.FilterAndSort.TestName)
		grip.Error(message.WrapError(err, message.Fields{
			"request":   gimlet.GetRequestID(ctx),
			"method":    "GET",
			"route":     "/test_results/test_name/{task_id}/{test_name}",
			"task_id":   h.opts.TaskID,
			"test_name": h.opts.FilterAndSort.TestName,
		}))
		return gimlet.MakeJSONInternalErrorResponder(err)
	}

	if len(testResults.Results) == 0 {
		return gimlet.MakeJSONErrorResponder(gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    "test result not found",
		})
	}
	return gimlet.NewJSONResponse(&testResults.Results[0])
}
