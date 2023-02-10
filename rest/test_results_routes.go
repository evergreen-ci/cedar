package rest

import (
	"context"
	"encoding/json"
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

type testResultsTasksBaseHandler struct {
	sc      data.Connector
	payload struct {
		TaskOpts   []data.TestResultsTaskOptions         `json:"tasks"`
		FilterOpts *data.TestResultsFilterAndSortOptions `json:"filters"`
	}
}

func (h *testResultsTasksBaseHandler) Parse(_ context.Context, r *http.Request) error {
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
	testResultsTasksBaseHandler
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
	testResultsTasksBaseHandler
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
	testResultsTasksBaseHandler
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

// TODO (EVG-18798): Remove these route handlers once Spruce and Evergreen are
// updated.

type testResultsBaseHandler struct {
	taskOpts   data.TestResultsTaskOptions
	filterOpts *data.TestResultsFilterAndSortOptions
}

// Parse fetches the task ID from the HTTP request.
func (h *testResultsBaseHandler) Parse(_ context.Context, r *http.Request) error {
	h.taskOpts.TaskID = gimlet.GetVars(r)["task_id"]

	vals := r.URL.Query()
	if len(vals[execution]) > 0 {
		exec, err := strconv.Atoi(vals[execution][0])
		if err != nil {
			return err
		}
		h.taskOpts.Execution = utility.ToIntPtr(exec)
	}
	if vals.Get(isDisplayTask) == trueString {
		h.taskOpts.DisplayTask = true
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

// Parse fetches the task ID from the HTTP request and any filter, sort, or
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

	h.filterOpts = &data.TestResultsFilterAndSortOptions{
		TestName: testName,
		Statuses: statuses,
		GroupID:  groupID,
		SortBy:   sortBy,
		Limit:    limit,
		Page:     page,
	}
	if vals.Get(testResultsSortDSC) == trueString {
		h.filterOpts.SortOrderDSC = true
	}
	if baseTaskID != "" {
		h.filterOpts.BaseTasks = []data.TestResultsTaskOptions{
			{
				TaskID:      baseTaskID,
				DisplayTask: h.taskOpts.DisplayTask,
			},
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

// Run finds and returns the desired test result based on the task ID.
func (h *testResultsGetByTaskIDHandler) Run(ctx context.Context) gimlet.Responder {
	testResults, err := h.sc.FindTestResults(ctx, []data.TestResultsTaskOptions{h.taskOpts}, h.filterOpts)
	if err != nil {
		err = errors.Wrapf(err, "getting test results by task ID '%s'", h.taskOpts.TaskID)
		logFindError(err, message.Fields{
			"request":    gimlet.GetRequestID(ctx),
			"method":     "GET",
			"route":      "/test_results/task_id/{task_id}",
			"task_id":    h.taskOpts.TaskID,
			"is_display": h.taskOpts.DisplayTask,
		})
		return gimlet.MakeJSONErrorResponder(err)
	}

	var resp gimlet.Responder
	resp = gimlet.NewJSONResponse(testResults)
	if h.filterOpts != nil && h.filterOpts.Limit > 0 {
		pages := &gimlet.ResponsePages{
			Prev: &gimlet.Page{
				BaseURL:         h.sc.GetBaseURL(),
				KeyQueryParam:   testResultsPage,
				LimitQueryParam: testResultsLimit,
				Key:             fmt.Sprintf("%d", h.filterOpts.Page),
				Limit:           h.filterOpts.Limit,
				Relation:        "prev",
			},
		}
		if len(testResults.Results) > 0 {
			pages.Next = &gimlet.Page{
				BaseURL:         h.sc.GetBaseURL(),
				KeyQueryParam:   testResultsPage,
				LimitQueryParam: testResultsLimit,
				Key:             fmt.Sprintf("%d", h.filterOpts.Page+1),
				Limit:           h.filterOpts.Limit,
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
	stats, err := h.sc.FindTestResultsStats(ctx, []data.TestResultsTaskOptions{h.taskOpts})
	if err != nil {
		err = errors.Wrapf(err, "getting test results stats by task ID '%s'", h.taskOpts.TaskID)
		logFindError(err, message.Fields{
			"request":         gimlet.GetRequestID(ctx),
			"method":          "GET",
			"route":           "/test_results/task_id/{task_id}/stats",
			"task_id":         h.taskOpts.TaskID,
			"is_display_task": h.taskOpts.DisplayTask,
		})
		return gimlet.MakeJSONInternalErrorResponder(err)
	}

	return gimlet.NewJSONResponse(stats)
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
	sample, err := h.sc.FindFailedTestResultsSample(ctx, []data.TestResultsTaskOptions{h.taskOpts})
	if err != nil {
		err = errors.Wrapf(err, "getting failed test results sample by task ID '%s'", h.taskOpts.TaskID)
		logFindError(err, message.Fields{
			"request":         gimlet.GetRequestID(ctx),
			"method":          "GET",
			"route":           "/test_results/task_id/{task_id}/failed_sample",
			"task_id":         h.taskOpts.TaskID,
			"is_display_task": h.taskOpts.DisplayTask,
		})
		return gimlet.MakeJSONInternalErrorResponder(err)
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
