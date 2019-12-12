package data

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/gimlet"
	"github.com/pkg/errors"
)

/////////////////////////////
// DBConnector Implementation
/////////////////////////////

// EvergreenProxyAuthLogRead sends a http request to Evergreen to check if the
// given user is authorized to read the log belonging to the given project. If
// the user is authorized, nil is returned, otherwise a gimlet.Responder with
// the appropriate http error code is returned.
func (dbc *DBConnector) EvergreenProxyAuthLogRead(ctx context.Context, evgConf *model.EvergreenConfig, userToken, resourceId string) gimlet.Responder {
	urlString := fmt.Sprintf("%s?resource=%s&resource_type=project&permission=project_logs&required_level=10", evgConf.URL, resourceId)
	req, err := http.NewRequest(http.MethodGet, urlString, nil)
	if err != nil {
		return gimlet.MakeJSONInternalErrorResponder(errors.Wrap(err, "error creating http request"))
	}
	req = req.WithContext(ctx)
	req.AddCookie(&http.Cookie{
		Name:   evgConf.AuthTokenCookie,
		Value:  userToken,
		Domain: evgConf.Domain,
	})

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

///////////////////////////////
// MockConnector Implementation
///////////////////////////////

// EvergreenProxyAuthLogRead checks if the given user exists in the mock
// connector, returning nil if they do and a gimlet.Responder with
// http.StatusUnauthorized otherwise.
func (mc *MockConnector) EvergreenProxyAuthLogRead(ctx context.Context, _ *model.EvergreenConfig, userToken, resourceId string) gimlet.Responder {
	if !mc.Users[userToken] {
		return gimlet.MakeTextErrorResponder(gimlet.ErrorResponse{
			StatusCode: http.StatusUnauthorized,
			Message:    fmt.Sprintf("unauthorized to read logs from project '%s'", resourceId),
		})
	}

	return nil
}
