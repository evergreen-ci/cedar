package perf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/evergreen-ci/cedar/model"
	"io"
	"net/http"
	"time"

	"github.com/evergreen-ci/utility"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

// ProxyService is the interface for the Proxy Service.
type ProxyService interface {
	ReportNewPerformanceDataAvailability(context.Context, model.PerformanceTestResultId) error
}

type proxyServiceClient struct {
	user    string
	token   string
	baseURL string
}

// NewProxyService creates a new ProxyService.
func NewProxyService(baseURL, user string, token string) ProxyService {
	return &proxyServiceClient{user: user, token: token, baseURL: baseURL}
}

// ReportNewPerformanceDataAvailability takes a PerformanceTestResultId and tries to report its data to a ProxyService.
func (spc *proxyServiceClient) ReportNewPerformanceDataAvailability(ctx context.Context, data model.PerformanceTestResultId) error {
	startAt := time.Now()

	if err := spc.doRequest(http.MethodPost, spc.baseURL+"/rabbitmq/performance_data_update/patch", ctx, data); err != nil {
		grip.Error(message.WrapError(err, message.Fields{
			"message": "Failed to report new time series performance data availability to proxy service",
			"update":  data,
		}))
		return nil
	}

	grip.Debug(message.Fields{
		"message":       "Reported new time series performance data availability to the proxy service",
		"update":        data,
		"duration_secs": time.Since(startAt).Seconds(),
	})

	return nil
}

func (spc *proxyServiceClient) doRequest(method, route string, ctx context.Context, in interface{}) error {
	body, err := json.Marshal(in)
	if err != nil {
		grip.Error(message.Fields{
			"message": "JSON encoding failed",
			"cause":   errors.WithStack(err),
			"data":    in,
		})
		return nil
	}

	//conf := utility.HTTPRetryConfiguration{
	//	MaxRetries:      50,
	//	TemporaryErrors: true,
	//	MaxDelay:        30 * time.Second,
	//	BaseDelay:       50 * time.Millisecond,
	//	Methods: []string{
	//		http.MethodGet,
	//		http.MethodPost,
	//		http.MethodPut,
	//		http.MethodDelete,
	//		http.MethodPatch,
	//	},
	//	Statuses: []int{
	//		// status code for timeouts from ELB in AWS (Kanopy infrastructure)
	//		499,
	//		http.StatusBadGateway,
	//		http.StatusServiceUnavailable,
	//		http.StatusGatewayTimeout,
	//		http.StatusInsufficientStorage,
	//		http.StatusConflict,
	//		http.StatusRequestTimeout,
	//		http.StatusPreconditionFailed,
	//		http.StatusExpectationFailed,
	//	},
	//	Errors: []error{
	//		// If a connection gets cut by the ELB, sometimes the client doesn't get an actual error
	//		// The client only receives a nil body leading to an EOF
	//		io.EOF,
	//	},
	//}
	conf := utility.NewDefaultHTTPRetryConf()
	conf.Statuses = append(conf.Statuses, 499)
	conf.Errors = []error{
		// If a connection gets cut by the ELB, sometimes the client doesn't get an actual error
		// The client only receives a nil body leading to an EOF
		io.EOF,
	}
	client := utility.GetHTTPRetryableClient(conf)
	defer utility.PutHTTPClient(client)

	req, err := http.NewRequest(method, route, bytes.NewBuffer(body))
	if err != nil {
		grip.Error(message.Fields{
			"message":   "Failed to report new time series performance data availability to proxy service",
			"cause":     errors.WithStack(err),
			"url":       route,
			"auth_user": spc.user,
		})
		return nil
	}
	req = req.WithContext(ctx)
	req.Header.Add("Cookie", fmt.Sprintf("auth_user=%v;auth_token=%v", spc.user, spc.token))
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		grip.Error(message.Fields{
			"message":   "Failed to report new time series performance data availability to proxy service",
			"status":    http.StatusText(resp.StatusCode),
			"url":       route,
			"auth_user": spc.user,
		})
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		grip.Error(message.Fields{
			"message":   "Failed to report new time series performance data availability to proxy service",
			"status":    http.StatusText(resp.StatusCode),
			"url":       route,
			"auth_user": spc.user,
		})
	}
	return nil
}
