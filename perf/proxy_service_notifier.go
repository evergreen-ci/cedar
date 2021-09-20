package perf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

// PerformanceAnalysisProxyService is the client interface for interacting with a performance analysis proxy service.
type PerformanceAnalysisProxyService interface {
	ReportNewPerformanceDataAvailability(context.Context, model.PerformanceAnalysisProxyServiceID) error
}

type performanceAnalysisProxyServiceClient struct {
	user    string
	token   string
	baseURL string
}

// NewPerformanceAnalysisProxyService creates a new performance analysis proxy service client.
func NewPerformanceAnalysisProxyService(options PerformanceAnalysisProxyServiceOptions) PerformanceAnalysisProxyService {
	return &performanceAnalysisProxyServiceClient{user: options.User, token: options.Token, baseURL: options.BaseURL}
}

// ReportNewPerformanceDataAvailability reports the given ID to the underlying performance analysis proxy service.
func (spc *performanceAnalysisProxyServiceClient) ReportNewPerformanceDataAvailability(ctx context.Context, data model.PerformanceAnalysisProxyServiceID) error {
	startAt := time.Now()

	if err := spc.doRequest(http.MethodPost, spc.baseURL+"/rabbitmq/performance_data_update/patch", ctx, data); err != nil {
		return errors.WithStack(err)
	}

	grip.Debug(message.Fields{
		"message":       "reported new time series performance data availability to the proxy service",
		"update":        data,
		"duration_secs": time.Since(startAt).Seconds(),
	})

	return nil
}

func (spc *performanceAnalysisProxyServiceClient) doRequest(method, route string, ctx context.Context, in interface{}) error {
	body, err := json.Marshal(in)
	if err != nil {
		return errors.Wrap(err, "json encoding failed")
	}
	conf := utility.NewDefaultHTTPRetryConf()
	conf.Statuses = append(conf.Statuses, 499)
	conf.MaxDelay = 30 * time.Second
	conf.Errors = []error{
		// If a connection gets cut by the ELB, sometimes the client doesn't get an actual error.
		// The client only receives a nil body leading to an EOF.
		io.EOF,
	}
	client := utility.GetHTTPRetryableClient(conf)
	defer utility.PutHTTPClient(client)

	req, err := http.NewRequest(method, route, bytes.NewBuffer(body))
	if err != nil {
		return errors.WithStack(err)
	}
	req = req.WithContext(ctx)
	req.Header.Add("Cookie", fmt.Sprintf("auth_user=%v;auth_token=%v", spc.user, spc.token))
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return errors.WithStack(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.Errorf("failed to report new time series performance data availability to proxy service, status: %q, url: %q, auth_user: %q", http.StatusText(resp.StatusCode), route, spc.user)
	}
	return nil
}

// MockPerformanceAnalysisProxyService is a mock performance analysis proxy service client for testing.
type MockPerformanceAnalysisProxyService struct {
	Calls []model.PerformanceAnalysisProxyServiceID
}

// ReportNewPerformanceDataAvailability appends the given ID to the mock implementation's calls cache.
func (m *MockPerformanceAnalysisProxyService) ReportNewPerformanceDataAvailability(_ context.Context, data model.PerformanceAnalysisProxyServiceID) error {
	m.Calls = append(m.Calls, data)
	return nil
}

// NewMockPerformanceAnalysisProxyService returns a mock performance analysis proxy service client.
func NewMockPerformanceAnalysisProxyService(mockProxyService *MockPerformanceAnalysisProxyService) func(options PerformanceAnalysisProxyServiceOptions) PerformanceAnalysisProxyService {
	return func(_ PerformanceAnalysisProxyServiceOptions) PerformanceAnalysisProxyService {
		return mockProxyService
	}
}

// PerformanceAnalysisProxyServiceOptions is the options struct for the performanceAnalysisProxyServiceClient.
type PerformanceAnalysisProxyServiceOptions struct {
	User    string
	Token   string
	BaseURL string
}
