package rest

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/gimlet"
	"github.com/pkg/errors"
)

type evgAuthReadLogByIDMiddleware struct {
	sc      data.Connector
	evgConf *model.EvergreenConfig
}

// NewEvgAuthReadLogByIDMiddlware returns an implementation of
// gimlet.Middleware that sends a http request to Evergreen to check if the
// user is authorized to read the log they are trying to access based on the
// given ID.
func NewEvgAuthReadLogByIDMiddleware(sc data.Connector, evgConf *model.EvergreenConfig) gimlet.Middleware {
	return &evgAuthReadLogByIDMiddleware{
		sc:      sc,
		evgConf: evgConf,
	}
}

func (m *evgAuthReadLogByIDMiddleware) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	ctx := r.Context()

	id := gimlet.GetVars(r)["id"]
	apiLog, err := m.sc.FindLogMetadataByID(ctx, id)
	if err != nil {
		gimlet.WriteResponse(rw, gimlet.MakeJSONErrorResponder(err))
		return
	}

	if resp := evgAuthReadLog(ctx, r, m.evgConf, *apiLog.Info.Project); resp != nil {
		gimlet.WriteResponse(rw, resp)
		return
	}

	next(rw, r)
}

type evgAuthReadLogByTaskIDMiddleware struct {
	sc      data.Connector
	evgConf *model.EvergreenConfig
}

// NewEvgAuthReadLogByTaskIDMiddlware returns an implementation of
// gimlet.Middleware that sends a http request to Evergreen to check if the
// user is authorized to read the log they are trying to access based on the
// given task ID.
func NewEvgAuthReadLogByTaskIDMiddleware(sc data.Connector, evgConf *model.EvergreenConfig) gimlet.Middleware {
	return &evgAuthReadLogByTaskIDMiddleware{
		sc:      sc,
		evgConf: evgConf,
	}
}

func (m *evgAuthReadLogByTaskIDMiddleware) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	ctx := r.Context()

	taskID := gimlet.GetVars(r)["task_id"]
	apiLogs, err := m.sc.FindLogMetadataByTaskID(ctx, data.BuildloggerOptions{TaskID: taskID})
	if err != nil {
		gimlet.WriteResponse(rw, gimlet.MakeJSONErrorResponder(err))
		return
	}

	if resp := evgAuthReadLog(ctx, r, m.evgConf, *apiLogs[0].Info.Project); resp != nil {
		gimlet.WriteResponse(rw, resp)
		return
	}

	next(rw, r)
}

func evgAuthReadLog(ctx context.Context, r *http.Request, evgConf *model.EvergreenConfig, resourceId string) gimlet.Responder {
	cookie, err := r.Cookie(evgConf.AuthTokenCookie)
	if err != nil {
		return gimlet.MakeTextErrorResponder(gimlet.ErrorResponse{
			StatusCode: http.StatusUnauthorized,
			Message:    "unauthorized user",
		})
	}

	urlString := fmt.Sprintf("%s/rest/v2/auth?resource=%s&resource_type=project&permission=project_logs&required_level=10", evgConf.URL, resourceId)
	req, err := http.NewRequest(http.MethodGet, urlString, nil)
	if err != nil {
		return gimlet.MakeJSONInternalErrorResponder(errors.Wrap(err, "error creating http request"))
	}
	req = req.WithContext(ctx)
	req.AddCookie(cookie)

	client := util.GetDefaultHTTPRetryableClient()
	defer util.PutHTTPClient(client)

	resp, err := client.Do(req)
	if err != nil {
		return gimlet.MakeJSONInternalErrorResponder(errors.Wrap(err, "error authenticating user"))
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return gimlet.MakeTextErrorResponder(gimlet.ErrorResponse{
			StatusCode: http.StatusUnauthorized,
			Message:    "unauthorized user",
		})
	}

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return gimlet.MakeJSONInternalErrorResponder(errors.Wrap(err, "error reading response body"))
	}

	if string(bytes) != "true" {
		return gimlet.MakeTextErrorResponder(gimlet.ErrorResponse{
			StatusCode: http.StatusUnauthorized,
			Message:    fmt.Sprintf("unauthorized to read logs from project '%s'", resourceId),
		})
	}

	return nil
}
