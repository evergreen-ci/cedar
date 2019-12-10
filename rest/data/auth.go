package data

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/gimlet"
	"github.com/jpillora/backoff"
	"github.com/pkg/errors"
)

/////////////////////////////
// DBConnector Implementation
/////////////////////////////

// EvergreenProxyAuthLogRead sends a http request to Evergreen to check if the
// given user is authorized to read the log belonging to the given project. If
// the user is authorized, nil is returned, otherwise a gimlet.Responder with
// the appropriate http error code is returned.
func (dbc *DBConnector) EvergreenProxyAuthLogRead(ctx context.Context, userToken, baseURL, resourceId string) gimlet.Responder {
	urlString := fmt.Sprintf("%s?resource=%s&resource_type=project&permission=project_logs&required_level=10", baseURL, resourceId)
	req, err := http.NewRequest(http.MethodGet, urlString, nil)
	if err != nil {
		return gimlet.MakeJSONInternalErrorResponder(errors.Wrap(err, "error creating http request"))
	}
	req = req.WithContext(ctx)
	req.Header = map[string][]string{"mci-token": {userToken}}

	client := util.GetHTTPClient()
	defer util.PutHTTPClient(client)

	b := &backoff.Backoff{
		Min:    100 * time.Millisecond,
		Max:    10 * time.Second,
		Factor: 2,
	}
	var responder gimlet.Responder
	for i := 0; i < 10; i++ {
		resp, err := client.Do(req)
		if err != nil {
			responder = gimlet.MakeJSONInternalErrorResponder(errors.Wrap(err, "error authenticating user"))
			time.Sleep(b.Duration())
			continue
		}
		if resp.StatusCode == http.StatusUnauthorized {
			return gimlet.MakeTextErrorResponder(gimlet.ErrorResponse{
				StatusCode: http.StatusUnauthorized,
				Message:    "unauthorized user",
			})
		}

		bytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			responder = gimlet.MakeJSONInternalErrorResponder(errors.Wrap(err, "error reading response body"))
			time.Sleep(b.Duration())
			continue
		}

		if string(bytes) != "true" {
			return gimlet.MakeTextErrorResponder(gimlet.ErrorResponse{
				StatusCode: http.StatusUnauthorized,
				Message:    fmt.Sprintf("unauthorized to read logs from project '%s'", resourceId),
			})
		}
		return nil
	}

	return responder
}

///////////////////////////////
// MockConnector Implementation
///////////////////////////////

// EvergreenProxyAuthLogRead checks if the given user exists in the mock
// connector, returning nil if they do and a gimlet.Responder with
// http.StatusUnauthorized otherwise.
func (mc *MockConnector) EvergreenProxyAuthLogRead(ctx context.Context, userToken, _, resourceId string) gimlet.Responder {
	if !mc.Users[userToken] {
		return gimlet.MakeTextErrorResponder(gimlet.ErrorResponse{
			StatusCode: http.StatusUnauthorized,
			Message:    fmt.Sprintf("unauthorized to read logs from project '%s'", resourceId),
		})
	}

	return nil
}
