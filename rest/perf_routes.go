package rest

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

const (
	perfStartAt = "started_after"
	perfEndAt   = "finished_before"
)

// timeRangeFormatYearMonthDay represents the time range format rounded to the
// nearest day (YYYY-MM-DD).
const timeRangeFormatYearMonthDay = "2006-01-02"

///////////////////////////////////////////////////////////////////////////////
//
// GET /perf/{id}

type perfGetByIdHandler struct {
	id string
	sc data.Connector
}

func makeGetPerfById(sc data.Connector) gimlet.RouteHandler {
	return &perfGetByIdHandler{
		sc: sc,
	}
}

// Factory returns a pointer to a new perfGetByIdHandler.
func (h *perfGetByIdHandler) Factory() gimlet.RouteHandler {
	return &perfGetByIdHandler{
		sc: h.sc,
	}
}

// Parse fetches the id from the http request.
func (h *perfGetByIdHandler) Parse(_ context.Context, r *http.Request) error {
	h.id = gimlet.GetVars(r)["id"]
	return nil
}

// Run calls the data FindPerformanceResultById function and returns the
// PerformanceResult from the provider.
func (h *perfGetByIdHandler) Run(ctx context.Context) gimlet.Responder {
	perfResult, err := h.sc.FindPerformanceResultById(ctx, h.id)
	if err != nil {
		err = errors.Wrapf(err, "problem getting performance result by id '%s'", h.id)
		grip.Error(message.WrapError(err, message.Fields{
			"request": gimlet.GetRequestID(ctx),
			"method":  "GET",
			"route":   "/perf/{id}",
			"id":      h.id,
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}
	return gimlet.NewJSONResponse(perfResult)
}

///////////////////////////////////////////////////////////////////////////////
//
// DELETE /perf/{id}

type perfRemoveByIdHandler struct {
	id string
	sc data.Connector
}

func makeRemovePerfById(sc data.Connector) gimlet.RouteHandler {
	return &perfRemoveByIdHandler{
		sc: sc,
	}
}

// Factory returns a pointer to a new perfRemoveByIdHandler.
func (h *perfRemoveByIdHandler) Factory() gimlet.RouteHandler {
	return &perfRemoveByIdHandler{
		sc: h.sc,
	}
}

// Parse fetches the id from the http request.
func (h *perfRemoveByIdHandler) Parse(_ context.Context, r *http.Request) error {
	h.id = gimlet.GetVars(r)["id"]
	return nil
}

// Run calls the data RemovePerformanceResultById function and returns the
// error.
func (h *perfRemoveByIdHandler) Run(ctx context.Context) gimlet.Responder {
	numRemoved, err := h.sc.RemovePerformanceResultById(ctx, h.id)
	if err != nil {
		err = errors.Wrapf(err, "problem removing performance result by id '%s'", h.id)
		grip.Error(message.WrapError(err, message.Fields{
			"request": gimlet.GetRequestID(ctx),
			"method":  "DELETE",
			"route":   "/perf/{id}",
			"id":      h.id,
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}
	return gimlet.NewJSONResponse(fmt.Sprintf("Delete operation removed %d performance results", numRemoved))
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /perf/task_id/{task_id}

type perfGetByTaskIdHandler struct {
	opts data.PerformanceOptions
	sc   data.Connector
}

func makeGetPerfByTaskId(sc data.Connector) gimlet.RouteHandler {
	return &perfGetByTaskIdHandler{
		sc: sc,
	}
}

// Factory returns a pointer to a new perfGetByTaskIdHandler.
func (h *perfGetByTaskIdHandler) Factory() gimlet.RouteHandler {
	return &perfGetByTaskIdHandler{
		sc: h.sc,
	}
}

// Parse fetches the task_id from the http request.
func (h *perfGetByTaskIdHandler) Parse(_ context.Context, r *http.Request) error {
	h.opts.TaskID = gimlet.GetVars(r)["task_id"]
	h.opts.Tags = r.URL.Query()["tags"]
	return nil
}

// Run calls the data FindPerformanceResults function and returns the
// PerformanceResults from the provider.
func (h *perfGetByTaskIdHandler) Run(ctx context.Context) gimlet.Responder {
	perfResults, err := h.sc.FindPerformanceResults(ctx, h.opts)
	if err != nil {
		err = errors.Wrapf(err, "problem getting performance results by task id '%s'", h.opts.TaskID)
		grip.Error(message.WrapError(err, message.Fields{
			"request": gimlet.GetRequestID(ctx),
			"method":  "GET",
			"route":   "/perf/task_id/{task_id}",
			"task_id": h.opts.TaskID,
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}
	return gimlet.NewJSONResponse(perfResults)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /perf/task_name/{task_name}

type perfGetByTaskNameHandler struct {
	opts data.PerformanceOptions
	sc   data.Connector
}

func makeGetPerfByTaskName(sc data.Connector) gimlet.RouteHandler {
	return &perfGetByTaskNameHandler{
		sc: sc,
	}
}

// Factory returns a pointer to a new perfGetByTaskNameHandler.
func (h *perfGetByTaskNameHandler) Factory() gimlet.RouteHandler {
	return &perfGetByTaskNameHandler{
		sc: h.sc,
	}
}

// Parse fetches the task_name from the http request.
func (h *perfGetByTaskNameHandler) Parse(_ context.Context, r *http.Request) error {
	h.opts.TaskName = gimlet.GetVars(r)["task_name"]
	vals := r.URL.Query()
	h.opts.Project = vals.Get("project")
	h.opts.Variant = vals.Get("variant")
	h.opts.Tags = vals["tags"]
	catcher := grip.NewBasicCatcher()
	var err error
	h.opts.Interval, err = parseTimeRange(timeRangeFormatYearMonthDay, vals.Get(perfStartAt), vals.Get(perfEndAt))
	catcher.Add(errors.Wrap(err, "invalid time range"))
	catcher.NewWhen(h.opts.Interval.IsZero(), "time interval must have start and end")
	catcher.NewWhen(!h.opts.Interval.IsValid(), "time interval must have a start time before its end time")
	limit := vals.Get("limit")
	if limit != "" {
		h.opts.Limit, err = strconv.Atoi(limit)
		catcher.Add(errors.Wrap(err, "invalid limit"))
		catcher.NewWhen(h.opts.Limit < 0, "cannot have negative limit")
	} else {
		h.opts.Limit = 0
	}
	return catcher.Resolve()
}

// Run calls the data FindPerformanceResults function and returns the
// PerformanceResults from the provider.
func (h *perfGetByTaskNameHandler) Run(ctx context.Context) gimlet.Responder {
	perfResults, err := h.sc.FindPerformanceResults(ctx, h.opts)
	if err != nil {
		err = errors.Wrapf(err, "problem getting performance results by task_name '%s'", h.opts.TaskName)
		grip.Error(message.WrapError(err, message.Fields{
			"request":   gimlet.GetRequestID(ctx),
			"method":    "GET",
			"route":     "/perf/task_name/{task_name}",
			"task_name": h.opts.TaskName,
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}
	return gimlet.NewJSONResponse(perfResults)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /perf/version/{version}

type perfGetByVersionHandler struct {
	opts data.PerformanceOptions
	sc   data.Connector
}

func makeGetPerfByVersion(sc data.Connector) gimlet.RouteHandler {
	return &perfGetByVersionHandler{
		sc: sc,
	}
}

// Factory returns a pointer to a new perfGetByVersionHandler.
func (h *perfGetByVersionHandler) Factory() gimlet.RouteHandler {
	return &perfGetByVersionHandler{
		sc: h.sc,
	}
}

// Parse fetches the version from the http request.
func (h *perfGetByVersionHandler) Parse(_ context.Context, r *http.Request) error {
	h.opts.Version = gimlet.GetVars(r)["version"]
	h.opts.Tags = r.URL.Query()["tags"]
	return nil
}

// Run calls the data FindPerformanceResults function returns the
// PerformanceResult from the provider.
func (h *perfGetByVersionHandler) Run(ctx context.Context) gimlet.Responder {
	perfResults, err := h.sc.FindPerformanceResults(ctx, h.opts)
	if err != nil {
		err = errors.Wrapf(err, "problem getting performance results by version '%s'", h.opts.Version)
		grip.Error(message.WrapError(err, message.Fields{
			"request": gimlet.GetRequestID(ctx),
			"method":  "GET",
			"route":   "/perf/version/{version}",
			"version": h.opts.Version,
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}
	return gimlet.NewJSONResponse(perfResults)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /perf/children/{id}

type perfGetChildrenHandler struct {
	id       string
	maxDepth int
	tags     []string
	sc       data.Connector
}

func makeGetPerfChildren(sc data.Connector) gimlet.RouteHandler {
	return &perfGetChildrenHandler{
		sc: sc,
	}
}

// Factory returns a pointer to a new perfGetChildrenHandler.
func (h *perfGetChildrenHandler) Factory() gimlet.RouteHandler {
	return &perfGetChildrenHandler{
		sc: h.sc,
	}
}

// Parse fetches the id from the http request.
func (h *perfGetChildrenHandler) Parse(_ context.Context, r *http.Request) error {
	h.id = gimlet.GetVars(r)["id"]
	vals := r.URL.Query()
	h.tags = vals["tags"]
	var err error
	h.maxDepth, err = strconv.Atoi(vals.Get("max_depth"))
	return errors.Wrap(err, "invalid max depth")
}

// Run calls the data FindPerformanceResultWithChildren function and returns
// the PerformanceResults from the provider.
func (h *perfGetChildrenHandler) Run(ctx context.Context) gimlet.Responder {
	perfResults, err := h.sc.FindPerformanceResultWithChildren(ctx, h.id, h.maxDepth, h.tags...)
	if err != nil {
		err = errors.Wrapf(err, "problem getting performance result and children by id '%s'", h.id)
		grip.Error(message.WrapError(err, message.Fields{
			"request": gimlet.GetRequestID(ctx),
			"method":  "GET",
			"route":   "/perf/children/{id}",
			"id":      h.id,
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}
	return gimlet.NewJSONResponse(perfResults)
}

///////////////////////////////////////////////////////////////////////////////
//
// POST /perf/signal_processing/recalculate

type perfSignalProcessingRecalculateHandler struct {
	sc data.Connector
}

func makePerfSignalProcessingRecalculate(sc data.Connector) gimlet.RouteHandler {
	return &perfSignalProcessingRecalculateHandler{
		sc: sc,
	}
}

func (h *perfSignalProcessingRecalculateHandler) Factory() gimlet.RouteHandler {
	return &perfSignalProcessingRecalculateHandler{
		sc: h.sc,
	}
}

func (h *perfSignalProcessingRecalculateHandler) Parse(_ context.Context, r *http.Request) error {
	return nil
}

func (h *perfSignalProcessingRecalculateHandler) Run(ctx context.Context) gimlet.Responder {
	err := h.sc.ScheduleSignalProcessingRecalculateJobs(ctx)
	if err != nil {
		err = errors.Wrapf(err, "Error scheduling signal processing recalculation jobs")
		grip.Error(message.WrapError(err, message.Fields{
			"request": gimlet.GetRequestID(ctx),
			"method":  "POST",
			"route":   "/perf/change_points",
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}
	return gimlet.NewJSONResponse(struct{}{})
}
