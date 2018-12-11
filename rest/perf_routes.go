package rest

import (
	"context"
	"net/http"

	"github.com/evergreen-ci/cedar/rest/data"
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
