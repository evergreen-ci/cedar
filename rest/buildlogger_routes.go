package rest

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

const (
	logStartAt    = "start"
	logEndAt      = "end"
	execution     = "execution"
	procName      = "proc_name"
	tags          = "tags"
	printTime     = "print_time"
	printPriority = "print_priority"
	limit         = "limit"
	paginate      = "paginate"
	trueString    = "true"
	softSizeLimit = 10 * 1024 * 1024
	baseURL       = "https://cedar.mongodb.com"
)

///////////////////////////////////////////////////////////////////////////////
//
// GET /buildlogger/{id}

type logGetByIDHandler struct {
	opts data.BuildloggerOptions
	sc   data.Connector
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
	var err error
	catcher := grip.NewBasicCatcher()

	h.opts.ID = gimlet.GetVars(r)["id"]
	vals := r.URL.Query()
	h.opts.PrintTime = vals.Get(printTime) == trueString
	h.opts.PrintPriority = vals.Get(printPriority) == trueString
	h.opts.TimeRange, err = parseTimeRange(vals, logStartAt, logEndAt)
	catcher.Add(err)
	if len(vals[limit]) > 0 {
		h.opts.Limit, err = strconv.Atoi(vals[limit][0])
		catcher.Add(err)
	}
	if vals.Get(paginate) == trueString && h.opts.Limit <= 0 {
		h.opts.SoftSizeLimit = softSizeLimit
	}

	return catcher.Resolve()
}

// Run calls FindLogByID and returns the log.
func (h *logGetByIDHandler) Run(ctx context.Context) gimlet.Responder {
	data, next, paginated, err := h.sc.FindLogByID(ctx, h.opts)
	if err != nil {
		err = errors.Wrapf(err, "problem getting log by id '%s'", h.opts.ID)
		grip.Error(message.WrapError(err, message.Fields{
			"request": gimlet.GetRequestID(ctx),
			"method":  "GET",
			"route":   "/buildlogger/{id}",
			"id":      h.opts.ID,
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}

	return newBuildloggerResponder(data, h.opts.TimeRange.StartAt, next, paginated)
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
		err = errors.Wrapf(err, "problem getting log metadata by id '%s'", h.id)
		grip.Error(message.WrapError(err, message.Fields{
			"request": gimlet.GetRequestID(ctx),
			"method":  "GET",
			"route":   "/buildlogger/{id}/meta",
			"id":      h.id,
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}

	return gimlet.NewJSONResponse(apiLog)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /buildlogger/task_id/{task_id}

type logGetByTaskIDHandler struct {
	opts data.BuildloggerOptions
	sc   data.Connector
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

	h.opts.TaskID = gimlet.GetVars(r)["task_id"]
	vals := r.URL.Query()
	h.opts.ProcessName = vals.Get(procName)
	h.opts.Tags = vals[tags]
	h.opts.PrintTime = vals.Get(printTime) == trueString
	h.opts.PrintPriority = vals.Get(printPriority) == trueString
	h.opts.TimeRange, err = parseTimeRange(vals, logStartAt, logEndAt)
	catcher.Add(err)
	if len(vals[execution]) > 0 {
		h.opts.Execution, err = strconv.Atoi(vals[execution][0])
		catcher.Add(err)
	} else {
		h.opts.EmptyExecution = true
	}
	if len(vals[limit]) > 0 {
		h.opts.Limit, err = strconv.Atoi(vals[limit][0])
		catcher.Add(err)
	}
	if len(vals["n"]) > 0 {
		h.opts.Tail, err = strconv.Atoi(vals["n"][0])
		catcher.Add(err)
	}
	if vals.Get(paginate) == trueString && h.opts.Limit <= 0 && h.opts.Tail <= 0 {
		h.opts.SoftSizeLimit = softSizeLimit
	}

	return catcher.Resolve()
}

// Run calls FindLogsByTaskID and returns the merged logs.
func (h *logGetByTaskIDHandler) Run(ctx context.Context) gimlet.Responder {
	data, next, paginated, err := h.sc.FindLogsByTaskID(ctx, h.opts)
	if err != nil {
		err = errors.Wrapf(err, "problem getting logs by task id '%s'", h.opts.TaskID)
		grip.Error(message.WrapError(err, message.Fields{
			"request": gimlet.GetRequestID(ctx),
			"method":  "GET",
			"route":   "/buildlogger/task_id/{task_id}",
			"task_id": h.opts.TaskID,
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}

	return newBuildloggerResponder(data, h.opts.TimeRange.StartAt, next, paginated)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /buildlogger/task_id/{task_id}/meta

type logMetaGetByTaskIDHandler struct {
	opts data.BuildloggerOptions
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
	h.opts.TaskID = gimlet.GetVars(r)["task_id"]
	vals := r.URL.Query()
	h.opts.Tags = vals[tags]
	if len(vals[execution]) > 0 {
		var err error
		h.opts.Execution, err = strconv.Atoi(vals[execution][0])
		return err
	} else {
		h.opts.EmptyExecution = true
	}

	return nil
}

// Run calls FindLogMetadataByTaskID and returns the merged logs.
func (h *logMetaGetByTaskIDHandler) Run(ctx context.Context) gimlet.Responder {
	apiLogs, err := h.sc.FindLogMetadataByTaskID(ctx, h.opts)
	if err != nil {
		err = errors.Wrapf(err, "problem getting log metadata by task id '%s'", h.opts.TaskID)
		grip.Error(message.WrapError(err, message.Fields{
			"request": gimlet.GetRequestID(ctx),
			"method":  "GET",
			"route":   "/buildlogger/task_id/{task_id}/meta",
			"task_id": h.opts.TaskID,
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}

	return gimlet.NewJSONResponse(apiLogs)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /buildlogger/test_name/{task_id}/{test_name}

type logGetByTestNameHandler struct {
	opts data.BuildloggerOptions
	sc   data.Connector
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
	catcher := grip.NewBasicCatcher()
	var err error

	h.opts.TaskID = gimlet.GetVars(r)["task_id"]
	h.opts.TestName = gimlet.GetVars(r)["test_name"]
	vals := r.URL.Query()
	h.opts.ProcessName = vals.Get(procName)
	h.opts.Tags = vals[tags]
	h.opts.PrintTime = vals.Get(printTime) == trueString
	h.opts.PrintPriority = vals.Get(printPriority) == trueString
	h.opts.TimeRange, err = parseTimeRange(vals, logStartAt, logEndAt)
	catcher.Add(err)
	if len(vals[execution]) > 0 {
		h.opts.Execution, err = strconv.Atoi(vals[execution][0])
		catcher.Add(err)
	} else {
		h.opts.EmptyExecution = true
	}
	if len(vals[limit]) > 0 {
		h.opts.Limit, err = strconv.Atoi(vals[limit][0])
		catcher.Add(err)
	}
	if vals.Get(paginate) == trueString && h.opts.Limit <= 0 {
		h.opts.SoftSizeLimit = softSizeLimit
	}

	return catcher.Resolve()
}

// Run calls FindLogsByTestName and returns the merged logs.
func (h *logGetByTestNameHandler) Run(ctx context.Context) gimlet.Responder {
	data, next, paginated, err := h.sc.FindLogsByTestName(ctx, h.opts)
	if err != nil {
		err = errors.Wrapf(err, "problem getting logs by test name '%s'", h.opts.TestName)
		grip.Error(message.WrapError(err, message.Fields{
			"request":   gimlet.GetRequestID(ctx),
			"method":    "GET",
			"route":     "/buildlogger/test_name/{task_id}/{test_name}",
			"task_id":   h.opts.TaskID,
			"test_name": h.opts.TestName,
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}

	return newBuildloggerResponder(data, h.opts.TimeRange.StartAt, next, paginated)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /buildlogger/test_name/{task_id}/{test_name}/meta

type logMetaGetByTestNameHandler struct {
	opts data.BuildloggerOptions
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
	h.opts.TaskID = gimlet.GetVars(r)["task_id"]
	h.opts.TestName = gimlet.GetVars(r)["test_name"]
	vals := r.URL.Query()
	h.opts.Tags = vals[tags]
	if len(vals[execution]) > 0 {
		var err error
		h.opts.Execution, err = strconv.Atoi(vals[execution][0])
		return err
	} else {
		h.opts.EmptyExecution = true
	}

	return nil
}

// Run calls FindLogMetadataByTestName and returns the merged logs.
func (h *logMetaGetByTestNameHandler) Run(ctx context.Context) gimlet.Responder {
	testLogs, err := h.sc.FindLogMetadataByTestName(ctx, h.opts)
	if err != nil {
		err = errors.Wrapf(err, "problem getting log metadata by test name '%s'", h.opts.TestName)
		grip.Error(message.WrapError(err, message.Fields{
			"request":   gimlet.GetRequestID(ctx),
			"method":    "GET",
			"route":     "/buildlogger/test_name/{task_id}/{test_name}/meta",
			"task_id":   h.opts.TaskID,
			"test_name": h.opts.TestName,
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}
	h.opts.TestName = ""
	globalLogs, err := h.sc.FindLogMetadataByTestName(ctx, h.opts)
	errResp, ok := err.(gimlet.ErrorResponse)
	if err != nil && (!ok || errResp.StatusCode != http.StatusNotFound) {
		err = errors.Wrapf(err, "problem getting log metadata by test name '%s'", h.opts.TestName)
		grip.Error(message.WrapError(err, message.Fields{
			"request":   gimlet.GetRequestID(ctx),
			"method":    "GET",
			"route":     "/buildlogger/test_name/{task_id}/{test_name}/meta",
			"task_id":   h.opts.TaskID,
			"test_name": h.opts.TestName,
		}))
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting log metadata by test name '%s'", h.opts.TestName))
	}

	return gimlet.NewJSONResponse(append(testLogs, globalLogs...))
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /buildlogger/test_name/{task_id}/{test_name}/group/{group_id}

type logGroupHandler struct {
	opts    data.BuildloggerOptions
	groupID string
	sc      data.Connector
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
	catcher := grip.NewBasicCatcher()

	h.opts.TaskID = gimlet.GetVars(r)["task_id"]
	h.opts.TestName = gimlet.GetVars(r)["test_name"]
	h.groupID = gimlet.GetVars(r)["group_id"]
	vals := r.URL.Query()
	h.opts.Tags = append(vals[tags], h.groupID)
	h.opts.PrintTime = vals.Get(printTime) == trueString
	h.opts.PrintPriority = vals.Get(printPriority) == trueString
	if len(vals[execution]) > 0 {
		h.opts.Execution, err = strconv.Atoi(vals[execution][0])
		catcher.Add(err)
	} else {
		h.opts.EmptyExecution = true
	}
	if vals.Get(logStartAt) != "" || vals.Get(logEndAt) != "" {
		h.opts.TimeRange, err = parseTimeRange(vals, logStartAt, logEndAt)
		catcher.Add(err)
	}
	if len(vals[limit]) > 0 {
		h.opts.Limit, err = strconv.Atoi(vals[limit][0])
		catcher.Add(err)
	}
	if vals.Get(paginate) == trueString && h.opts.Limit <= 0 {
		h.opts.SoftSizeLimit = softSizeLimit
	}

	return catcher.Resolve()
}

// Run calls FindGroupedLogs and returns the merged logs.
func (h *logGroupHandler) Run(ctx context.Context) gimlet.Responder {
	if h.opts.TimeRange.IsZero() {
		testLogs, err := h.sc.FindLogMetadataByTestName(ctx, h.opts)
		if err != nil {
			err = errors.Wrapf(err, "problem getting log metadata by test name '%s'", h.opts.TestName)
			grip.Error(message.WrapError(err, message.Fields{
				"request":   gimlet.GetRequestID(ctx),
				"method":    "GET",
				"route":     "/buildlogger/test_name/{task_id}/{test_name}/group/{group_id}",
				"task_id":   h.opts.TaskID,
				"test_name": h.opts.TestName,
				"group_id":  h.groupID,
			}))
			return gimlet.MakeJSONErrorResponder(err)
		}

		for _, log := range testLogs {
			if h.opts.TimeRange.StartAt.After(time.Time(log.CreatedAt)) || h.opts.TimeRange.StartAt.IsZero() {
				h.opts.TimeRange.StartAt = time.Time(log.CreatedAt)
			}
			if h.opts.TimeRange.EndAt.Before(time.Time(log.CompletedAt)) {
				h.opts.TimeRange.EndAt = time.Time(log.CompletedAt)
			}
		}
	}

	data, next, paginated, err := h.sc.FindGroupedLogs(ctx, h.opts)
	if err != nil {
		err = errors.Wrapf(err, "problem getting grouped logs with task_id/test_name/group_id '%s/%s/%s'",
			h.opts.TaskID, h.opts.TestName, h.groupID)
		grip.Error(message.WrapError(err, message.Fields{
			"request":   gimlet.GetRequestID(ctx),
			"method":    "GET",
			"route":     "/buildlogger/test_name/{task_id}/{test_name}/group/{group_id}",
			"task_id":   h.opts.TaskID,
			"test_name": h.opts.TestName,
			"group_id":  h.groupID,
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}

	return newBuildloggerResponder(data, h.opts.TimeRange.StartAt, next, paginated)
}

func newBuildloggerResponder(data []byte, last, next time.Time, paginated bool) gimlet.Responder {
	resp := gimlet.NewTextResponse(data)

	if paginated {
		pages := &gimlet.ResponsePages{
			Prev: &gimlet.Page{
				BaseURL:         baseURL,
				KeyQueryParam:   "start",
				LimitQueryParam: "limit",
				Key:             last.Format(time.RFC3339Nano),
				Relation:        "prev",
			},
		}
		if !next.IsZero() {
			pages.Next = &gimlet.Page{
				BaseURL:         baseURL,
				KeyQueryParam:   "start",
				LimitQueryParam: "limit",
				Key:             next.Format(time.RFC3339Nano),
				Relation:        "next",
			}
		}

		if err := resp.SetPages(pages); err != nil {
			return gimlet.MakeJSONErrorResponder(errors.Wrap(err, "problem setting response pages"))
		}
	}

	return resp
}
