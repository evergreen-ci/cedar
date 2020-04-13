package rest

import (
	"context"
	"net/http"
	"strconv"

	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

///////////////////////////////////////////////////////////////////////////////
//
// GET /perf/project/{projectId}/version/change_points

type perfGetChangesPointByVersionHandler struct {
	page      int
	pageSize  int
	projectId string
	sc        data.Connector
}

func makeGetChangePointsByVersion(sc data.Connector) gimlet.RouteHandler {
	return &perfGetChangesPointByVersionHandler{
		sc: sc,
	}
}

// Factory returns a pointer to a new perfGetChangesPointByVersionHandler.
func (h *perfGetChangesPointByVersionHandler) Factory() gimlet.RouteHandler {
	return &perfGetChangesPointByVersionHandler{
		sc: h.sc,
	}
}

// Parse fetches the id from the http request.
func (h *perfGetChangesPointByVersionHandler) Parse(_ context.Context, r *http.Request) error {
	h.projectId = gimlet.GetVars(r)["projectId"]
	vals := r.URL.Query()
	catcher := grip.NewBasicCatcher()
	var err error
	page := vals.Get("page")
	if page != "" {
		h.page, err = strconv.Atoi(page)
		catcher.Add(err)
	} else {
		h.page = 0
	}
	pageSize := vals.Get("page_size")
	if pageSize != "" {
		h.pageSize, err = strconv.Atoi(pageSize)
		catcher.Add(err)
	} else {
		h.pageSize = 0
	}
	return catcher.Resolve()
}

// Run calls FindLogMetadataByID and returns the log.
func (h *perfGetChangesPointByVersionHandler) Run(ctx context.Context) gimlet.Responder {
	changePointsByVersion, err := h.sc.GetChangePointsByVersions(ctx, h.projectId, h.page, h.page)
	if err != nil {
		err = errors.Wrapf(err, "problem getting change points by version for project '%s'", h.projectId)
		grip.Error(message.WrapError(err, message.Fields{
			"request": gimlet.GetRequestID(ctx),
			"method":  "GET",
			"route":   "/perf/project/{projectId}/version/change_points",
			"id":      h.projectId,
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}
	return gimlet.NewJSONResponse(changePointsByVersion)
}
