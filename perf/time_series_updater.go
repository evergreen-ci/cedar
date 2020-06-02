package perf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/evergreen-ci/utility"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

type ArgumentsModel struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

type TimeSeriesDataModel struct {
	PerformanceResultID string  `json:"cedar_perf_result_id"`
	Order               int     `json:"order"`
	Value               float64 `json:"value"`
	Version             string  `json:"version"`
}

type TimeSeriesModel struct {
	Project     string                `json:"project"`
	Variant     string                `json:"variant"`
	Task        string                `json:"task"`
	Test        string                `json:"test"`
	Measurement string                `json:"measurement"`
	Arguments   []ArgumentsModel      `json:"args"`
	Data        []TimeSeriesDataModel `json:"data"`
}

type PerformanceAnalysisService interface {
	ReportUpdatedTimeSeries(context.Context, TimeSeriesModel) error
}

type performanceAnalysisAndTriageClient struct {
	user    string
	token   string
	baseURL string
}

func NewPerformanceAnalysisService(baseURL, user string, token string) PerformanceAnalysisService {
	return &performanceAnalysisAndTriageClient{user: user, token: token, baseURL: baseURL}
}

func (spc *performanceAnalysisAndTriageClient) ReportUpdatedTimeSeries(ctx context.Context, series TimeSeriesModel) error {
	startAt := time.Now()

	if err := spc.doRequest(http.MethodPost, spc.baseURL+"/time_series/update", ctx, series); err != nil {
		return errors.WithStack(err)
	}

	grip.Debug(message.Fields{
		"message":       "Reported updated time series to performance analysis and triage service",
		"update":        series,
		"duration_secs": time.Since(startAt).Seconds(),
	})

	return nil
}

func (spc *performanceAnalysisAndTriageClient) doRequest(method, route string, ctx context.Context, in interface{}) error {
	body, err := json.Marshal(in)
	if err != nil {
		return errors.WithStack(err)
	}

	conf := utility.HTTPRetryConfiguration{
		MaxRetries:      50,
		TemporaryErrors: true,
		MaxDelay:        30 * time.Second,
		BaseDelay:       50 * time.Millisecond,
		Methods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodPatch,
		},
		Statuses: []int{
			// status code for timeouts from ELB in AWS (Kanopy infrastructure)
			499,
			http.StatusBadGateway,
			http.StatusServiceUnavailable,
			http.StatusGatewayTimeout,
			http.StatusInsufficientStorage,
			http.StatusConflict,
			http.StatusRequestTimeout,
			http.StatusPreconditionFailed,
			http.StatusExpectationFailed,
		},
		Errors: []error{
			// If a connection gets cut by the ELB, sometimes the client doesn't get an actual error
			// The client only receives a nil body leading to an EOF
			io.EOF,
		},
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
		grip.Warning(message.Fields{
			"message":   "Failed to report time series update to performance analysis and triage service",
			"status":    http.StatusText(resp.StatusCode),
			"url":       route,
			"auth_user": spc.user,
		})
		return errors.Errorf("Failed to report time series update to performance analysis and triage service, status: %q, url: %q, auth_user: %q", http.StatusText(resp.StatusCode), route, spc.user)
	}
	return nil
}
