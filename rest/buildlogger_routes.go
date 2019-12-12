package rest

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

const (
	logStartAt = "start"
	logEndAt   = "end"
	tags       = "tags"
	printTime  = "printTime"
)

///////////////////////////////////////////////////////////////////////////////
//
// GET /buildlogger/{id}

type logGetByIDHandler struct {
	id        string
	tr        util.TimeRange
	printTime bool
	sc        data.Connector
	userToken string
	evgConf   *model.EvergreenConfig
}

func makeGetLogByID(sc data.Connector, evgConf *model.EvergreenConfig) gimlet.RouteHandler {
	return &logGetByIDHandler{
		sc:      sc,
		evgConf: evgConf,
	}
}

// Factory returns a pointer to a new logGetByIDHandler.
func (h *logGetByIDHandler) Factory() gimlet.RouteHandler {
	return &logGetByIDHandler{
		sc:      h.sc,
		evgConf: h.evgConf,
	}
}

// Parse fetches the id, auth token cookie, time range, and print time flag
// from the http request.
func (h *logGetByIDHandler) Parse(_ context.Context, r *http.Request) error {
	h.id = gimlet.GetVars(r)["id"]

	var cookie *http.Cookie
	var err error
	cookie, err = r.Cookie(h.evgConf.AuthTokenCookie)
	if err == nil {
		h.userToken = cookie.Value
	}

	vals := r.URL.Query()
	h.printTime = vals.Get(printTime) == "true"
	h.tr, err = parseTimeRange(vals, logStartAt, logEndAt)

	return err
}

// Run calls FindLogByID and returns the log.
func (h *logGetByIDHandler) Run(ctx context.Context) gimlet.Responder {
	apiLog, err := h.sc.FindLogMetadataByID(ctx, h.id)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting log metadata by id '%s'", h.id))
	}
	resp := h.sc.EvergreenProxyAuthLogRead(ctx, h.evgConf, h.userToken, *apiLog.Info.Project)
	if resp != nil {
		return resp
	}

	r, err := h.sc.FindLogByID(ctx, h.id, h.tr, h.printTime)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting log by id '%s'", h.id))
	}

	return gimlet.NewTextResponse(r)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /buildlogger/{id}/meta

type logMetaGetByIDHandler struct {
	id        string
	sc        data.Connector
	userToken string
	evgConf   *model.EvergreenConfig
}

func makeGetLogMetaByID(sc data.Connector, evgConf *model.EvergreenConfig) gimlet.RouteHandler {
	return &logMetaGetByIDHandler{
		sc:      sc,
		evgConf: evgConf,
	}
}

// Factory returns a pointer to a new logMetaGetByIDHandler.
func (h *logMetaGetByIDHandler) Factory() gimlet.RouteHandler {
	return &logMetaGetByIDHandler{
		sc:      h.sc,
		evgConf: h.evgConf,
	}
}

// Parse fetches the id and auth token cookie from the http request.
func (h *logMetaGetByIDHandler) Parse(_ context.Context, r *http.Request) error {
	h.id = gimlet.GetVars(r)["id"]

	cookie, err := r.Cookie(h.evgConf.AuthTokenCookie)
	if err == nil {
		h.userToken = cookie.Value
	}

	return nil
}

// Run calls FindLogMetadataByID and returns the log.
func (h *logMetaGetByIDHandler) Run(ctx context.Context) gimlet.Responder {
	apiLog, err := h.sc.FindLogMetadataByID(ctx, h.id)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting log metadata by id '%s'", h.id))
	}

	resp := h.sc.EvergreenProxyAuthLogRead(ctx, h.evgConf, h.userToken, *apiLog.Info.Project)
	if resp != nil {
		return resp
	}

	return gimlet.NewJSONResponse(apiLog)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /buildlogger/task_id/{task_id}

type logGetByTaskIDHandler struct {
	id        string
	tags      []string
	tr        util.TimeRange
	n         int
	printTime bool
	sc        data.Connector
	userToken string
	evgConf   *model.EvergreenConfig
}

func makeGetLogByTaskID(sc data.Connector, evgConf *model.EvergreenConfig) gimlet.RouteHandler {
	return &logGetByTaskIDHandler{
		sc:      sc,
		evgConf: evgConf,
	}
}

// Factory returns a pointer to a new logGetByTaskIDHandler.
func (h *logGetByTaskIDHandler) Factory() gimlet.RouteHandler {
	return &logGetByTaskIDHandler{
		sc:      h.sc,
		evgConf: h.evgConf,
	}
}

// Parse fetches the id, auth token cookie, time range, tags, print time flag,
// and number of tail lines from the http request.
func (h *logGetByTaskIDHandler) Parse(_ context.Context, r *http.Request) error {
	h.id = gimlet.GetVars(r)["task_id"]

	var cookie *http.Cookie
	var err error
	cookie, err = r.Cookie(h.evgConf.AuthTokenCookie)
	if err == nil {
		h.userToken = cookie.Value
	}

	catcher := grip.NewBasicCatcher()
	vals := r.URL.Query()
	h.tags = vals[tags]
	h.printTime = vals.Get(printTime) == "true"
	h.tr, err = parseTimeRange(vals, logStartAt, logEndAt)
	catcher.Add(err)
	if len(vals["n"]) > 0 {
		h.n, err = strconv.Atoi(vals["n"][0])
		catcher.Add(err)
	}

	return catcher.Resolve()
}

// Run calls FindLogsByTaskID and returns the merged logs.
func (h *logGetByTaskIDHandler) Run(ctx context.Context) gimlet.Responder {
	apiLogs, err := h.sc.FindLogMetadataByTaskID(ctx, h.id)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting log metadata by task id '%s'", h.id))
	}
	resp := h.sc.EvergreenProxyAuthLogRead(ctx, h.evgConf, h.userToken, *apiLogs[0].Info.Project)
	if resp != nil {
		return resp
	}

	r, err := h.sc.FindLogsByTaskID(ctx, h.id, h.tr, h.n, h.printTime, h.tags...)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting logs by task id '%s'", h.id))
	}

	return gimlet.NewTextResponse(r)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /buildlogger/task_id/{task_id}/meta

type logMetaGetByTaskIDHandler struct {
	id        string
	tags      []string
	sc        data.Connector
	userToken string
	evgConf   *model.EvergreenConfig
}

func makeGetLogMetaByTaskID(sc data.Connector, evgConf *model.EvergreenConfig) gimlet.RouteHandler {
	return &logMetaGetByTaskIDHandler{
		sc:      sc,
		evgConf: evgConf,
	}
}

// Factory returns a pointer to a new logMetaGetByTaskIDHandler.
func (h *logMetaGetByTaskIDHandler) Factory() gimlet.RouteHandler {
	return &logMetaGetByTaskIDHandler{
		sc:      h.sc,
		evgConf: h.evgConf,
	}
}

// Parse fetches the id and auth token cookie from the http request.
func (h *logMetaGetByTaskIDHandler) Parse(_ context.Context, r *http.Request) error {
	h.id = gimlet.GetVars(r)["task_id"]

	cookie, err := r.Cookie(h.evgConf.AuthTokenCookie)
	if err == nil {
		h.userToken = cookie.Value
	}

	vals := r.URL.Query()
	h.tags = vals[tags]

	return nil
}

// Run calls FindLogMetadataByTaskID and returns the merged logs.
func (h *logMetaGetByTaskIDHandler) Run(ctx context.Context) gimlet.Responder {
	apiLogs, err := h.sc.FindLogMetadataByTaskID(ctx, h.id, h.tags...)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting log metadata by task id '%s'", h.id))
	}

	resp := h.sc.EvergreenProxyAuthLogRead(ctx, h.evgConf, h.userToken, *apiLogs[0].Info.Project)
	if resp != nil {
		return resp
	}

	return gimlet.NewJSONResponse(apiLogs)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /buildlogger/test_name/{task_id}/{test_name}

type logGetByTestNameHandler struct {
	id        string
	name      string
	tags      []string
	tr        util.TimeRange
	printTime bool
	sc        data.Connector
	userToken string
	evgConf   *model.EvergreenConfig
}

func makeGetLogByTestName(sc data.Connector, evgConf *model.EvergreenConfig) gimlet.RouteHandler {
	return &logGetByTestNameHandler{
		sc:      sc,
		evgConf: evgConf,
	}
}

// Factory returns a pointer to a new logGetByTestNameHandler.
func (h *logGetByTestNameHandler) Factory() gimlet.RouteHandler {
	return &logGetByTestNameHandler{
		sc:      h.sc,
		evgConf: h.evgConf,
	}
}

// Parse fetches the id, name, auth token cookie, time range, tags, and print
// time flag from the http request.
func (h *logGetByTestNameHandler) Parse(_ context.Context, r *http.Request) error {
	h.id = gimlet.GetVars(r)["task_id"]
	h.name = gimlet.GetVars(r)["test_name"]

	var cookie *http.Cookie
	var err error
	cookie, err = r.Cookie(h.evgConf.AuthTokenCookie)
	if err == nil {
		h.userToken = cookie.Value
	}

	vals := r.URL.Query()
	h.tags = vals[tags]
	h.printTime = vals.Get(printTime) == "true"
	h.tr, err = parseTimeRange(vals, logStartAt, logEndAt)

	return err
}

// Run calls FindLogsByTestName and returns the merged logs.
func (h *logGetByTestNameHandler) Run(ctx context.Context) gimlet.Responder {
	apiLogs, err := h.sc.FindLogMetadataByTaskID(ctx, h.id)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting log metadata by task id '%s'", h.id))
	}
	resp := h.sc.EvergreenProxyAuthLogRead(ctx, h.evgConf, h.userToken, *apiLogs[0].Info.Project)
	if resp != nil {
		return resp
	}

	r, err := h.sc.FindLogsByTestName(ctx, h.id, h.name, h.tr, h.printTime, h.tags...)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting logs by test name '%s'", h.name))
	}

	return gimlet.NewTextResponse(r)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /buildlogger/test_name/{task_id}/{test_name}/meta

type logMetaGetByTestNameHandler struct {
	id        string
	name      string
	tags      []string
	sc        data.Connector
	userToken string
	evgConf   *model.EvergreenConfig
}

func makeGetLogMetaByTestName(sc data.Connector, evgConf *model.EvergreenConfig) gimlet.RouteHandler {
	return &logMetaGetByTestNameHandler{
		sc:      sc,
		evgConf: evgConf,
	}
}

// Factory returns a pointer to a new logMetaGetByTestNameHandler.
func (h *logMetaGetByTestNameHandler) Factory() gimlet.RouteHandler {
	return &logMetaGetByTestNameHandler{
		sc:      h.sc,
		evgConf: h.evgConf,
	}
}

// Parse fetches the id, name, auth token cookie, and tags from the http
// request.
func (h *logMetaGetByTestNameHandler) Parse(_ context.Context, r *http.Request) error {
	h.id = gimlet.GetVars(r)["task_id"]
	h.name = gimlet.GetVars(r)["test_name"]

	cookie, err := r.Cookie(h.evgConf.AuthTokenCookie)
	if err == nil {
		h.userToken = cookie.Value
	}

	vals := r.URL.Query()
	h.tags = vals[tags]

	return nil
}

// Run calls FindLogMetadataByTestName and returns the merged logs.
func (h *logMetaGetByTestNameHandler) Run(ctx context.Context) gimlet.Responder {
	testLogs, err := h.sc.FindLogMetadataByTestName(ctx, h.id, h.name, h.tags...)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting log metadata by test name '%s'", h.name))
	}

	resp := h.sc.EvergreenProxyAuthLogRead(ctx, h.evgConf, h.userToken, *testLogs[0].Info.Project)
	if resp != nil {
		return resp
	}

	globalLogs, err := h.sc.FindLogMetadataByTestName(ctx, h.id, "", h.tags...)
	errResp, ok := err.(gimlet.ErrorResponse)
	if err != nil && (!ok || errResp.StatusCode == http.StatusNotFound) {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting log metadata by test name '%s'", h.name))
	}

	return gimlet.NewJSONResponse(append(testLogs, globalLogs...))
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /buildlogger/test_name/{task_id}/{test_name}/group/{group_id}

type logGroupHandler struct {
	id        string
	name      string
	groupID   string
	tags      []string
	tr        util.TimeRange
	printTime bool
	sc        data.Connector
	userToken string
	evgConf   *model.EvergreenConfig
}

func makeGetLogGroup(sc data.Connector, evgConf *model.EvergreenConfig) gimlet.RouteHandler {
	return &logGroupHandler{
		sc:      sc,
		evgConf: evgConf,
	}
}

// Factory returns a pointer to a new logGetByGroupHandler.
func (h *logGroupHandler) Factory() gimlet.RouteHandler {
	return &logGroupHandler{
		sc:      h.sc,
		evgConf: h.evgConf,
	}
}

// Parse fetches the id, name, auth token cookie, time range, tags, and print
// time flag from the http request.
func (h *logGroupHandler) Parse(_ context.Context, r *http.Request) error {
	h.id = gimlet.GetVars(r)["task_id"]
	h.name = gimlet.GetVars(r)["test_name"]
	h.groupID = gimlet.GetVars(r)["group_id"]

	var cookie *http.Cookie
	var err error
	cookie, err = r.Cookie(h.evgConf.AuthTokenCookie)
	if err == nil {
		h.userToken = cookie.Value
	} else {
		err = nil
	}

	vals := r.URL.Query()
	h.tags = vals[tags]
	h.printTime = vals.Get(printTime) == "true"
	if vals.Get(logStartAt) != "" || vals.Get(logEndAt) != "" {
		h.tr, err = parseTimeRange(vals, logStartAt, logEndAt)
	}

	return err
}

// Run calls FindGroupedLogs and returns the merged logs.
func (h *logGroupHandler) Run(ctx context.Context) gimlet.Responder {
	var project string
	if h.tr.IsZero() {
		testLogs, err := h.sc.FindLogMetadataByTestName(ctx, h.id, h.name, append(h.tags, h.groupID)...)
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

		project = *testLogs[0].Info.Project
	}

	if project == "" {
		apiLogs, err := h.sc.FindLogMetadataByTaskID(ctx, h.id)
		if err != nil {
			return gimlet.MakeJSONErrorResponder(errors.Wrapf(err, "Error getting log metadata by task id '%s'", h.id))
		}
		project = *apiLogs[0].Info.Project
	}
	resp := h.sc.EvergreenProxyAuthLogRead(ctx, h.evgConf, h.userToken, project)
	if resp != nil {
		return resp
	}

	r, err := h.sc.FindGroupedLogs(ctx, h.id, h.name, h.groupID, h.tr, h.printTime, h.tags...)
	if err != nil {
		return gimlet.MakeJSONErrorResponder(errors.Wrapf(err,
			"Error getting grouped logs with task_id/test_name/group_id '%s/%s/%s'", h.id, h.name, h.groupID))
	}

	return gimlet.NewTextResponse(r)
}
