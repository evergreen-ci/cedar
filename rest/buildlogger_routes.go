package rest

import (
	"context"
	"net/http"
	"strings"
	"time"

	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/gimlet"
	"github.com/pkg/errors"
)

const (
	logStartAt = "start"
	logEndAt   = "end"
)

///////////////////////////////////////////////////////////////////////////////
//
// GET /buildlogger/{id}

type logGetByIDHandler struct {
	id string
	tr util.TimeRange
	sc data.Connector
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
	h.tr, err = parseTimeRange(vals, logStartAt, logEndAt)

	return err
}

// Run calls FindLogByID and returns the log.
func (h *logGetByIDHandler) Run(ctx context.Context) gimlet.Responder {
	it, err := h.sc.FindLogByID(ctx, h.id, h.tr)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting log by id '%s'", h.id))
	}

	return gimlet.NewTextResponse(dbModel.NewLogIteratorReader(ctx, it))
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /buildlogger/meta/{id}

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
	id   string
	tags []string
	tr   util.TimeRange
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

	h.id = gimlet.GetVars(r)["id"]
	vals := r.URL.Query()
	h.tags = vals["tags"]
	h.tr, err = parseTimeRange(vals, logStartAt, logEndAt)

	return err
}

// Run calls FindLogsByTaskID and returns the merged logs.
func (h *logGetByTaskIDHandler) Run(ctx context.Context) gimlet.Responder {
	it, err := h.sc.FindLogsByTaskID(ctx, h.id, h.tr, h.tags...)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting logs by task id '%s'", h.id))
	}

	return gimlet.NewTextResponse(dbModel.NewLogIteratorReader(ctx, it))
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /buildlogger/meta/task_id/{task_id}

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
	h.id = gimlet.GetVars(r)["id"]
	vals := r.URL.Query()
	h.tags = vals["tags"]

	return nil
}

// Run calls FindLogMetadataByTaskID and returns the merged logs.
func (h *logMetaGetByTaskIDHandler) Run(ctx context.Context) gimlet.Responder {
	apiLogs, err := h.sc.FindLogMetadataByTaskID(ctx, h.id, h.tags...)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting log metadata by task id '%s'", h.id))
	}

	return gimlet.NewJSONResponse(apiLogs)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /buildlogger/test_name/{task_id}/{test_name}

type logGetByTestNameHandler struct {
	id   string
	name string
	tags []string
	tr   util.TimeRange
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
	var err error

	h.id = gimlet.GetVars(r)["id"]
	h.name = gimlet.GetVars(r)["name"]
	vals := r.URL.Query()
	h.tags = vals["tags"]
	if vals.Get(logStartAt) != "" || vals.Get(logEndAt) != "" {
		h.tr, err = parseTimeRange(vals, logStartAt, logEndAt)
	}

	return err
}

// Run calls FindLogsByTestName and returns the merged logs.
func (h *logGetByTestNameHandler) Run(ctx context.Context) gimlet.Responder {
	its := []dbModel.LogIterator{}

	if h.tr.IsZero() {
		testLogs, err := h.sc.FindLogMetadataByTestName(ctx, h.id, h.name, h.tags...)
		if err != nil {
			return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting log metadata by test name '%s'", h.name))
		}

		for _, log := range testLogs {
			if h.tr.StartAt.After(time.Time(log.CreatedAt)) || h.tr.StartAt.IsZero() {
				h.tr.StartAt = time.Time(log.CreatedAt)
			}
			if h.tr.EndAt.Before(time.Time(log.CompletedAt)) {
				h.tr.EndAt = time.Time(log.CompletedAt)
			}
		}
	}

	it, err := h.sc.FindLogsByTestName(ctx, h.id, h.name, h.tr, h.tags...)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting logs by test name '%s'", h.name))
	}
	its = append(its, it)

	it, err = h.sc.FindLogsByTestName(ctx, h.id, "", h.tr, h.tags...)
	if err == nil {
		its = append(its, it)
	} else if !strings.Contains(err.Error(), http.StatusText(http.StatusNotFound)) {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting logs by test name '%s'", h.name))
	}

	return gimlet.NewTextResponse(dbModel.NewLogIteratorReader(ctx, dbModel.NewMergingIterator(ctx, its...)))
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /buildlogger/meta/test_name/{task_id}/{test_name}

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
	h.id = gimlet.GetVars(r)["id"]
	vals := r.URL.Query()
	h.tags = vals["tags"]

	return nil
}

// Run calls FindLogMetadataByTestName and returns the merged logs.
func (h *logMetaGetByTestNameHandler) Run(ctx context.Context) gimlet.Responder {
	testLogs, err := h.sc.FindLogMetadataByTestName(ctx, h.id, h.name, h.tags...)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting log metadata by test name '%s'", h.name))
	}
	globalLogs, err := h.sc.FindLogMetadataByTestName(ctx, h.id, "", h.tags...)
	if err != nil && !strings.Contains(err.Error(), http.StatusText(http.StatusNotFound)) {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting log metadata by test name '%s'", h.name))
	}

	return gimlet.NewJSONResponse(append(testLogs, globalLogs...))
}
