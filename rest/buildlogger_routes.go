package rest

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

const (
	logStartAt = "start"
	logEndAt   = "end"
	execution  = "execution"
	procName   = "proc_name"
	tags       = "tags"
	printTime  = "print_time"
)

///////////////////////////////////////////////////////////////////////////////
//
// GET /buildlogger/{id}

type logGetByIDHandler struct {
	id        string
	tr        util.TimeRange
	printTime bool
	sc        data.Connector
}

func makeGetLogByID(sc data.Connector) gimlet.RouteHandler {
	return &logGetByIDHandler{
		sc: sc,
	}
}

// Factory returns a pointer to a new logGetByIDHandler.
func (h *logGetByIDHandler) Factory() gimlet.RouteHandler {
	return &logGetByIDHandler{
		sc: h.sc,
	}
}

// Parse fetches the id and time range from the http request.
func (h *logGetByIDHandler) Parse(_ context.Context, r *http.Request) error {
	h.id = gimlet.GetVars(r)["id"]

	vals := r.URL.Query()
	var err error
	h.printTime = vals.Get(printTime) == "true"
	h.tr, err = parseTimeRange(vals, logStartAt, logEndAt)

	return err
}

// Run calls FindLogByID and returns the log.
func (h *logGetByIDHandler) Run(ctx context.Context) gimlet.Responder {
	opts := data.BuildloggerOptions{
		ID:        h.id,
		TimeRange: h.tr,
		PrintTime: h.printTime,
	}
	r, err := h.sc.FindLogByID(ctx, opts)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting log by id '%s'", h.id))
	}

	return gimlet.NewTextResponse(r)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /buildlogger/{id}/meta

type logMetaGetByIDHandler struct {
	id string
	sc data.Connector
}

func makeGetLogMetaByID(sc data.Connector) gimlet.RouteHandler {
	return &logMetaGetByIDHandler{
		sc: sc,
	}
}

// Factory returns a pointer to a new logMetaGetByIDHandler.
func (h *logMetaGetByIDHandler) Factory() gimlet.RouteHandler {
	return &logMetaGetByIDHandler{
		sc: h.sc,
	}
}

// Parse fetches the id from the http request.
func (h *logMetaGetByIDHandler) Parse(_ context.Context, r *http.Request) error {
	h.id = gimlet.GetVars(r)["id"]
	return nil
}

// Run calls FindLogMetadataByID and returns the log.
func (h *logMetaGetByIDHandler) Run(ctx context.Context) gimlet.Responder {
	apiLog, err := h.sc.FindLogMetadataByID(ctx, h.id)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting log metadata by id '%s'", h.id))
	}

	return gimlet.NewJSONResponse(apiLog)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /buildlogger/task_id/{task_id}

type logGetByTaskIDHandler struct {
	id        string
	procName  string
	execution int
	tags      []string
	tr        util.TimeRange
	n         int
	printTime bool
	sc        data.Connector
}

func makeGetLogByTaskID(sc data.Connector) gimlet.RouteHandler {
	return &logGetByTaskIDHandler{
		sc: sc,
	}
}

// Factory returns a pointer to a new logGetByTaskIDHandler.
func (h *logGetByTaskIDHandler) Factory() gimlet.RouteHandler {
	return &logGetByTaskIDHandler{
		sc: h.sc,
	}
}

// Parse fetches the id and time range from the http request.
func (h *logGetByTaskIDHandler) Parse(_ context.Context, r *http.Request) error {
	var err error
	catcher := grip.NewBasicCatcher()

	h.id = gimlet.GetVars(r)["task_id"]
	vals := r.URL.Query()
	h.procName = vals.Get(procName)
	h.tags = vals[tags]
	h.printTime = vals.Get(printTime) == "true"
	h.tr, err = parseTimeRange(vals, logStartAt, logEndAt)
	catcher.Add(err)
	if len(vals[execution]) > 0 {
		h.execution, err = strconv.Atoi(vals[execution][0])
		catcher.Add(err)
	}
	if len(vals["n"]) > 0 {
		h.n, err = strconv.Atoi(vals["n"][0])
		catcher.Add(err)
	}

	return catcher.Resolve()
}

// Run calls FindLogsByTaskID and returns the merged logs.
func (h *logGetByTaskIDHandler) Run(ctx context.Context) gimlet.Responder {
	opts := data.BuildloggerOptions{
		TaskID:      h.id,
		ProcessName: h.procName,
		Tags:        h.tags,
		TimeRange:   h.tr,
		PrintTime:   h.printTime,
		Tail:        h.n,
	}
	r, err := h.sc.FindLogsByTaskID(ctx, opts)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting logs by task id '%s'", h.id))
	}

	return gimlet.NewTextResponse(r)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /buildlogger/task_id/{task_id}/meta

type logMetaGetByTaskIDHandler struct {
	id   string
	tags []string
	sc   data.Connector
}

func makeGetLogMetaByTaskID(sc data.Connector) gimlet.RouteHandler {
	return &logMetaGetByTaskIDHandler{
		sc: sc,
	}
}

// Factory returns a pointer to a new logMetaGetByTaskIDHandler.
func (h *logMetaGetByTaskIDHandler) Factory() gimlet.RouteHandler {
	return &logMetaGetByTaskIDHandler{
		sc: h.sc,
	}
}

// Parse fetches the id from the http request.
func (h *logMetaGetByTaskIDHandler) Parse(_ context.Context, r *http.Request) error {
	h.id = gimlet.GetVars(r)["task_id"]
	vals := r.URL.Query()
	h.tags = vals[tags]

	return nil
}

// Run calls FindLogMetadataByTaskID and returns the merged logs.
func (h *logMetaGetByTaskIDHandler) Run(ctx context.Context) gimlet.Responder {
	opts := data.BuildloggerOptions{
		TaskID: h.id,
		Tags:   h.tags,
	}
	apiLogs, err := h.sc.FindLogMetadataByTaskID(ctx, opts)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting log metadata by task id '%s'", h.id))
	}

	return gimlet.NewJSONResponse(apiLogs)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /buildlogger/test_name/{task_id}/{test_name}

type logGetByTestNameHandler struct {
	id        string
	name      string
	tags      []string
	tr        util.TimeRange
	printTime bool
	sc        data.Connector
}

func makeGetLogByTestName(sc data.Connector) gimlet.RouteHandler {
	return &logGetByTestNameHandler{
		sc: sc,
	}
}

// Factory returns a pointer to a new logGetByTestNameHandler.
func (h *logGetByTestNameHandler) Factory() gimlet.RouteHandler {
	return &logGetByTestNameHandler{
		sc: h.sc,
	}
}

// Parse fetches the id, name, time range, and tags from the http request.
func (h *logGetByTestNameHandler) Parse(_ context.Context, r *http.Request) error {
	var err error

	h.id = gimlet.GetVars(r)["task_id"]
	h.name = gimlet.GetVars(r)["test_name"]
	vals := r.URL.Query()
	h.tags = vals[tags]
	h.printTime = vals.Get(printTime) == "true"
	h.tr, err = parseTimeRange(vals, logStartAt, logEndAt)

	return err
}

// Run calls FindLogsByTestName and returns the merged logs.
func (h *logGetByTestNameHandler) Run(ctx context.Context) gimlet.Responder {
	opts := data.BuildloggerOptions{
		TaskID:    h.id,
		TestName:  h.name,
		Tags:      h.tags,
		TimeRange: h.tr,
		PrintTime: h.printTime,
	}
	r, err := h.sc.FindLogsByTestName(ctx, opts)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting logs by test name '%s'", h.name))
	}

	return gimlet.NewTextResponse(r)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /buildlogger/test_name/{task_id}/{test_name}/meta

type logMetaGetByTestNameHandler struct {
	id   string
	name string
	tags []string
	sc   data.Connector
}

func makeGetLogMetaByTestName(sc data.Connector) gimlet.RouteHandler {
	return &logMetaGetByTestNameHandler{
		sc: sc,
	}
}

// Factory returns a pointer to a new logMetaGetByTestNameHandler.
func (h *logMetaGetByTestNameHandler) Factory() gimlet.RouteHandler {
	return &logMetaGetByTestNameHandler{
		sc: h.sc,
	}
}

// Parse fetches the id, name, and tags from the http request.
func (h *logMetaGetByTestNameHandler) Parse(_ context.Context, r *http.Request) error {
	h.id = gimlet.GetVars(r)["task_id"]
	h.name = gimlet.GetVars(r)["test_name"]
	vals := r.URL.Query()
	h.tags = vals[tags]

	return nil
}

// Run calls FindLogMetadataByTestName and returns the merged logs.
func (h *logMetaGetByTestNameHandler) Run(ctx context.Context) gimlet.Responder {
	opts := data.BuildloggerOptions{
		TaskID:   h.id,
		TestName: h.name,
		Tags:     h.tags,
	}
	testLogs, err := h.sc.FindLogMetadataByTestName(ctx, opts)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting log metadata by test name '%s'", h.name))
	}
	opts.TestName = ""
	globalLogs, err := h.sc.FindLogMetadataByTestName(ctx, opts)
	errResp, ok := err.(gimlet.ErrorResponse)
	if err != nil && (!ok || errResp.StatusCode == http.StatusNotFound) {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting log metadata by test name '%s'", h.name))
	}

	return gimlet.NewJSONResponse(append(testLogs, globalLogs...))
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /buildlogger/test_name/{task_id}/{test_name}/group/{group_id}

type logGroupHandler struct {
	id        string
	name      string
	groupID   string
	tags      []string
	tr        util.TimeRange
	printTime bool
	sc        data.Connector
}

func makeGetLogGroup(sc data.Connector) gimlet.RouteHandler {
	return &logGroupHandler{
		sc: sc,
	}
}

// Factory returns a pointer to a new logGetByGroupHandler.
func (h *logGroupHandler) Factory() gimlet.RouteHandler {
	return &logGroupHandler{
		sc: h.sc,
	}
}

// Parse fetches the id, name, time range, and tags from the http request.
func (h *logGroupHandler) Parse(_ context.Context, r *http.Request) error {
	var err error

	h.id = gimlet.GetVars(r)["task_id"]
	h.name = gimlet.GetVars(r)["test_name"]
	h.groupID = gimlet.GetVars(r)["group_id"]
	vals := r.URL.Query()
	h.tags = vals[tags]
	h.printTime = vals.Get(printTime) == "true"
	if vals.Get(logStartAt) != "" || vals.Get(logEndAt) != "" {
		h.tr, err = parseTimeRange(vals, logStartAt, logEndAt)
	}

	return err
}

// Run calls FindGroupedLogs and returns the merged logs.
func (h *logGroupHandler) Run(ctx context.Context) gimlet.Responder {
	opts := data.BuildloggerOptions{
		TaskID:    h.id,
		TestName:  h.name,
		Tags:      append(h.tags, h.groupID),
		TimeRange: h.tr,
		PrintTime: h.printTime,
	}
	if opts.TimeRange.IsZero() {
		testLogs, err := h.sc.FindLogMetadataByTestName(ctx, opts)
		if err != nil {
			return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting log metadata by test name '%s'", h.name))
		}

		for _, log := range testLogs {
			if opts.TimeRange.StartAt.After(time.Time(log.CreatedAt)) || opts.TimeRange.StartAt.IsZero() {
				opts.TimeRange.StartAt = time.Time(log.CreatedAt)
			}
			if opts.TimeRange.EndAt.Before(time.Time(log.CompletedAt)) {
				opts.TimeRange.EndAt = time.Time(log.CompletedAt)
			}
		}
	}

	r, err := h.sc.FindGroupedLogs(ctx, opts)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err,
			"Error getting grouped logs with task_id/test_name/group_id '%s/%s/%s'", h.id, h.name, h.groupID))
	}

	return gimlet.NewTextResponse(r)
}
