package rest

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

///////////////////////////////////////////////////////////////////////////////
//
// GET /perf/project/{projectID}/change_points_by_version

type perfGetChangePointsByVersionHandler struct {
	args data.GetChangePointsGroupedByVersionArgs
	sc   data.Connector
}

func makeGetChangePointsByVersion(sc data.Connector) gimlet.RouteHandler {
	return &perfGetChangePointsByVersionHandler{
		sc: sc,
	}
}

// Factory returns a pointer to a new perfGetChangePointsByVersionHandler.
func (h *perfGetChangePointsByVersionHandler) Factory() gimlet.RouteHandler {
	return &perfGetChangePointsByVersionHandler{
		sc: h.sc,
	}
}

// Parse fetches the id from the http request.
func (h *perfGetChangePointsByVersionHandler) Parse(_ context.Context, r *http.Request) error {
	h.args.ProjectId = gimlet.GetVars(r)["projectID"]
	vals := r.URL.Query()
	catcher := grip.NewBasicCatcher()
	var err error
	page := vals.Get("page")
	if page != "" {
		h.args.Page, err = strconv.Atoi(page)
		catcher.Add(err)
	} else {
		h.args.Page = 0
	}
	pageSize := vals.Get("pageSize")
	if pageSize != "" {
		h.args.PageSize, err = strconv.Atoi(pageSize)
		catcher.Add(err)
	} else {
		h.args.PageSize = 0
	}
	h.args.VariantRegex = vals.Get("variantRegex")
	h.args.VersionRegex = vals.Get("versionRegex")
	h.args.TaskRegex = vals.Get("taskRegex")
	h.args.TestRegex = vals.Get("testRegex")
	h.args.MeasurementRegex = vals.Get("measurementRegex")
	tls := vals.Get("threadLevels")
	if tls != "" {
		tlslice := strings.Split(tls, ",")
		for _, tl := range tlslice {
			intTl, err := strconv.Atoi(tl)
			catcher.Add(err)
			h.args.ThreadLevels = append(h.args.ThreadLevels, intTl)
		}
	}
	return catcher.Resolve()
}

// Run calls FindLogMetadataByID and returns the log.
func (h *perfGetChangePointsByVersionHandler) Run(ctx context.Context) gimlet.Responder {
	changePointsByVersion, err := h.sc.GetChangePointsByVersion(ctx, h.args)
	if err != nil {
		err = errors.Wrapf(err, "problem getting change points by version for project '%s'", h.args.ProjectId)
		grip.Error(message.WrapError(err, message.Fields{
			"request": gimlet.GetRequestID(ctx),
			"method":  "GET",
			"route":   "/perf/project/{projectID}/change_points_by_version",
			"id":      h.args.ProjectId,
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}
	return gimlet.NewJSONResponse(changePointsByVersion)
}
