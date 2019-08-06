package rest

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

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
func (h *perfGetByIdHandler) Parse(ctx context.Context, r *http.Request) error {
	h.id = gimlet.GetVars(r)["id"]
	return nil
}

// Run calls the data FindPerformanceResultById function and returns the
// PerformanceResult from the provider.
func (h *perfGetByIdHandler) Run(ctx context.Context) gimlet.Responder {
	perfResult, err := h.sc.FindPerformanceResultById(h.id)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting performance result by id '%s'", h.id))
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
func (h *perfRemoveByIdHandler) Parse(ctx context.Context, r *http.Request) error {
	h.id = gimlet.GetVars(r)["id"]
	return nil
}

// Run calls the data RemovePerformanceResultById function and returns the
// error.
func (h *perfRemoveByIdHandler) Run(ctx context.Context) gimlet.Responder {
	numRemoved, err := h.sc.RemovePerformanceResultById(h.id)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error removing performance result by id '%s'", h.id))
	}
	return gimlet.NewJSONResponse(fmt.Sprintf("Delete operation removed %d performance results", numRemoved))
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /perf/task_id/{task_id}

type perfGetByTaskIdHandler struct {
	taskId   string
	interval util.TimeRange
	tags     []string
	sc       data.Connector
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
func (h *perfGetByTaskIdHandler) Parse(ctx context.Context, r *http.Request) error {
	h.taskId = gimlet.GetVars(r)["task_id"]
	vals := r.URL.Query()
	h.tags = vals["tags"]
	var err error
	h.interval, err = parseInterval(vals)
	return err
}

// Run calls the data FindPerformanceResultsByTaskId and function returns the
// PerformanceResults from the provider.
func (h *perfGetByTaskIdHandler) Run(ctx context.Context) gimlet.Responder {
	perfResults, err := h.sc.FindPerformanceResultsByTaskId(h.taskId, h.interval, h.tags...)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting performance results by task_id '%s'", h.taskId))
	}
	return gimlet.NewJSONResponse(perfResults)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /perf/task_name/{task_name}

type perfGetByTaskNameHandler struct {
	taskName string
	project  string
	interval util.TimeRange
	tags     []string
	limit    int
	variant  string
	sc       data.Connector
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
func (h *perfGetByTaskNameHandler) Parse(ctx context.Context, r *http.Request) error {
	h.taskName = gimlet.GetVars(r)["task_name"]
	vals := r.URL.Query()
	h.tags = vals["tags"]
	h.variant = vals.Get("variant")
	h.project = vals.Get("project")
	var err error
	catcher := grip.NewBasicCatcher()
	h.interval, err = parseInterval(vals)
	catcher.Add(err)
	limit := vals.Get("limit")
	if limit != "" {
		h.limit, err = strconv.Atoi(limit)
		catcher.Add(err)
	} else {
		h.limit = 0
	}
	return catcher.Resolve()
}

// Run calls the data FindPerformanceResultsByTaskName function and returns the
// PerformanceResults from the provider.
func (h *perfGetByTaskNameHandler) Run(ctx context.Context) gimlet.Responder {
	perfResults, err := h.sc.FindPerformanceResultsByTaskName(h.project, h.taskName, h.variant, h.interval, h.limit, h.tags...)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting performance results by task_id '%s'", h.taskName))
	}
	return gimlet.NewJSONResponse(perfResults)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /perf/version/{version}

type perfGetByVersionHandler struct {
	version  string
	interval util.TimeRange
	tags     []string
	sc       data.Connector
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
func (h *perfGetByVersionHandler) Parse(ctx context.Context, r *http.Request) error {
	h.version = gimlet.GetVars(r)["version"]
	vals := r.URL.Query()
	h.tags = vals["tags"]
	var err error
	h.interval, err = parseInterval(vals)
	return err
}

// Run calls the data FindPerformanceResultsByVersion function returns the
// PerformanceResult from the provider.
func (h *perfGetByVersionHandler) Run(ctx context.Context) gimlet.Responder {
	perfResults, err := h.sc.FindPerformanceResultsByVersion(h.version, h.interval, h.tags...)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting performance results by version '%s'", h.version))
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
func (h *perfGetChildrenHandler) Parse(ctx context.Context, r *http.Request) error {
	h.id = gimlet.GetVars(r)["id"]
	vals := r.URL.Query()
	h.tags = vals["tags"]
	var err error
	h.maxDepth, err = strconv.Atoi(vals.Get("max_depth"))
	return errors.Wrap(err, "failed to parse request")
}

// Run calls the data FindPerformanceResultWithChildren function and returns
// the PerformanceResults from the provider.
func (h *perfGetChildrenHandler) Run(ctx context.Context) gimlet.Responder {
	perfResults, err := h.sc.FindPerformanceResultWithChildren(h.id, h.maxDepth, h.tags...)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting performance result and children by id '%s'", h.id))
	}
	return gimlet.NewJSONResponse(perfResults)
}

///////////////////////////////////////////////////////////////////////////////
//
// Helper functions

func parseInterval(vals url.Values) (util.TimeRange, error) {
	interval := util.TimeRange{}
	startedAfter := vals.Get("started_after")
	finishedBefore := vals.Get("finished_before")

	if startedAfter != "" {
		start, err := time.ParseInLocation(time.RFC3339, startedAfter, time.UTC)
		if err != nil {
			return util.TimeRange{}, errors.Errorf("problem parsing start time '%s'", startedAfter)
		}
		interval.StartAt = start
	}

	if finishedBefore != "" {
		end, err := time.ParseInLocation(time.RFC3339, finishedBefore, time.UTC)
		if err != nil {
			return util.TimeRange{}, errors.Errorf("problem parsing end time '%s'", finishedBefore)
		}
		interval.EndAt = end
	} else {
		interval.EndAt = time.Now()
	}
	return interval, nil
}
