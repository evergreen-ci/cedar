package rest

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/gimlet"
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

// Run calls the data FindPerformanceResultById function returns the
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
// GET /perf/task/{task_id}

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

// Run calls the data FindPerformanceResultsByTaskId function returns the
// PerformanceResult from the provider.
func (h *perfGetByTaskIdHandler) Run(ctx context.Context) gimlet.Responder {
	perfResults, err := h.sc.FindPerformanceResultsByTaskId(h.taskId, h.interval, h.tags...)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting performance results by task_id '%s'", h.taskId))
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
	return &perfGetByTaskIdHandler{
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

// helper functions
//
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
