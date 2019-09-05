package rest

import (
	"context"
	"net/http"

	"github.com/evergreen-ci/cedar/model"
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
// GET /log/{id}

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

	return gimlet.NewTextResponse(model.NewLogIteratorReader(ctx, it))
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /log/meta/{id}

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
// GET /log/task_id/{task_id}

type logGetByTaskIDHandler struct {
	id string
	tr util.TimeRange
	sc data.Connector
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
	h.tr, err = parseTimeRange(vals, logStartAt, logEndAt)

	return err
}

// Run calls FindLogsByTaskID and returns the merged logs.
func (h *logGetByTaskIDHandler) Run(ctx context.Context) gimlet.Responder {
	it, err := h.sc.FindLogsByTaskID(ctx, h.id, h.tr)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting logs by task id '%s'", h.id))
	}

	return gimlet.NewTextResponse(model.NewLogIteratorReader(ctx, it))
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /log/meta/task_id/{task_id}

type logMetaGetByTaskIDHandler struct {
	id string
	sc data.Connector
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

	return nil
}

// Run calls FindLogMetadataByTaskID and returns the merged logs.
func (h *logMetaGetByTaskIDHandler) Run(ctx context.Context) gimlet.Responder {
	apiLogs, err := h.sc.FindLogMetadataByTaskID(ctx, h.id)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting log metadata by task id '%s'", h.id))
	}

	return gimlet.NewJSONResponse(apiLogs)
}
