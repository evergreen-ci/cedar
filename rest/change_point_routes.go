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
	page             int
	pageSize         int
	variantRegex     string
	versionRegex     string
	taskRegex        string
	testRegex        string
	measurementRegex string
	threadLevels     []int
	projectId        string
	sc               data.Connector
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
	h.projectId = gimlet.GetVars(r)["projectID"]
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
	pageSize := vals.Get("pageSize")
	if pageSize != "" {
		h.pageSize, err = strconv.Atoi(pageSize)
		catcher.Add(err)
	} else {
		h.pageSize = 0
	}
	h.variantRegex = vals.Get("variantRegex")
	h.versionRegex = vals.Get("versionRegex")
	h.taskRegex = vals.Get("taskRegex")
	h.testRegex = vals.Get("testRegex")
	h.measurementRegex = vals.Get("measurementRegex")
	tls := vals.Get("threadLevels")
	if tls != "" {
		tlslice := strings.Split(tls, ",")
		for _, tl := range tlslice {
			intTl, err := strconv.Atoi(tl)
			catcher.Add(err)
			h.threadLevels = append(h.threadLevels, intTl)
		}
	}
	return catcher.Resolve()
}

// Run calls FindLogMetadataByID and returns the log.
func (h *perfGetChangePointsByVersionHandler) Run(ctx context.Context) gimlet.Responder {
	changePointsByVersion, err := h.sc.GetChangePointsByVersion(ctx, h.projectId, h.page, h.page, h.variantRegex, h.versionRegex, h.taskRegex, h.testRegex, h.measurementRegex, h.threadLevels)
	if err != nil {
		err = errors.Wrapf(err, "problem getting change points by version for project '%s'", h.projectId)
		grip.Error(message.WrapError(err, message.Fields{
			"request": gimlet.GetRequestID(ctx),
			"method":  "GET",
			"route":   "/perf/project/{projectID}/change_points_by_version",
			"id":      h.projectId,
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}
	return gimlet.NewJSONResponse(changePointsByVersion)
}
