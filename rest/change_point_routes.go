package rest

import (
	"context"
	"net/http"
	"regexp"
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
	args data.GetChangePointsGroupedByVersionOpts
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

const (
	pageString       string = "page"
	pageSizeString   string = "pageSize"
	variantRegex     string = "variantRegex"
	versionRegex     string = "versionRegex"
	taskRegex        string = "taskRegex"
	testRegex        string = "testRegex"
	measurementRegex string = "measurementRegex"
)

// Parse fetches the id from the http request.
func (h *perfGetChangePointsByVersionHandler) Parse(_ context.Context, r *http.Request) error {
	h.args.ProjectID = gimlet.GetVars(r)["projectID"]
	vals := r.URL.Query()
	catcher := grip.NewBasicCatcher()
	var err error
	page := vals.Get(pageString)
	delete(vals, pageString)
	if page != "" {
		h.args.Page, err = strconv.Atoi(page)
		catcher.Add(err)
	} else {
		h.args.Page = 0
	}
	pageSize := vals.Get(pageSizeString)
	delete(vals, pageSizeString)
	if pageSize != "" {
		h.args.PageSize, err = strconv.Atoi(pageSize)
		catcher.Add(err)
	} else {
		h.args.PageSize = 10
	}
	h.args.VariantRegex = vals.Get(variantRegex)
	delete(vals, variantRegex)
	h.args.VersionRegex = vals.Get(versionRegex)
	delete(vals, versionRegex)
	h.args.TaskRegex = vals.Get(taskRegex)
	delete(vals, taskRegex)
	h.args.TestRegex = vals.Get(testRegex)
	delete(vals, testRegex)
	h.args.MeasurementRegex = vals.Get(measurementRegex)
	delete(vals, measurementRegex)
	h.args.Arguments = map[string][]int{}
	for k, v := range vals {
		key := toSnakeCase(k)
		val := v[0]
		if val != "" {
			valslice := strings.Split(val, ",")
			for _, valEntry := range valslice {
				intVal, err := strconv.Atoi(valEntry)
				catcher.Add(err)
				h.args.Arguments[key] = append(h.args.Arguments[key], intVal)
			}
		}
	}
	return catcher.Resolve()
}


func toSnakeCase(str string) string {
	matchFirstCap := regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchAllCap := regexp.MustCompile("([a-z0-9])([A-Z])")
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}

func (h *perfGetChangePointsByVersionHandler) Run(ctx context.Context) gimlet.Responder {
	changePointsByVersion, err := h.sc.GetChangePointsByVersion(ctx, h.args)
	if err != nil {
		err = errors.Wrapf(err, "problem getting change points by version for project '%s'", h.args.ProjectID)
		grip.Error(message.WrapError(err, message.Fields{
			"request": gimlet.GetRequestID(ctx),
			"method":  "GET",
			"route":   "/perf/project/{projectID}/change_points_by_version",
			"id":      h.args.ProjectID,
		}))
		return gimlet.MakeJSONErrorResponder(err)
	}
	return gimlet.NewJSONResponse(changePointsByVersion)
}
