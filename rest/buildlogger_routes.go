package rest

import (
	"context"
	"net/http"

	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/gimlet"
	"github.com/pkg/errors"
)

///////////////////////////////////////////////////////////////////////////////
//
// GET /log/{id}

type logGetByIDHandler struct {
	id string
	tr timeRange
	sc data.Connector
}

func makeGetLogByID(sc data.Connector) gimlet.RouteHandler {
	return &logGetByIdHandler{
		sc: sc,
	}
}

// Factory returns a pointer to a new logsGetByIdHandler.
func (h *logGetByIDHandler) Factory() gimlet.RouteHandler {
	return &logGetByIdHandler{
		sc: h.sc,
	}
}

// Parse fetches the id from the http request.
func (h *logGetByIDHandler) Parse(_ context.Context, r *http.Request) error {
	h.id = gimlet.GetVars(r)["id"]

	vals := r.URL.Query()
	var err error
	h.tr, err = parseInterval(vals)

	return err
}

// Run calls FindLogByID and returns the log.
func (h *logGetByIDHandler) Run(ctx context.Context) gimlet.Responder {
	it, err := h.sc.FindLogByID(ctx, h.id)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting log by id '%s'", h.id))
	}

	return gimlet.NewTextResponse(model.NewLogIteratorReader(ctx, it))
}
