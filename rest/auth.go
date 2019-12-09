package rest

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/evergreen-ci/gimlet"
	"github.com/pkg/errors"
)

func evergreenProxyAuthLogRead(ctx context.Context, userToken, baseURL, resourceId string) gimlet.Responder {
	client := &http.Client{Timeout: time.Minute}
	urlString := fmt.Sprintf("%s?resource=%s&resource_type=project&permission=project_logs&required_level=10", baseURL, resourceId)
	req, err := http.NewRequest(http.MethodGet, urlString, nil)
	if err != nil {
		return gimlet.MakeJSONInternalErrorResponder(errors.Wrap(err, "error creating http request"))
	}
	req = req.WithContext(ctx)
	req.Header = map[string][]string{"mci-token": {userToken}}

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
