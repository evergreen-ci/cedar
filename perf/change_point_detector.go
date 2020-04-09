package perf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/gimlet"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

type AlgorithmConfigurationValue struct {
	Name  string
	Value interface{}
}

type Algorithm interface {
	Name() string
	Version() int
	Configuration() []AlgorithmConfigurationValue
}

type eDivisiveMeans struct {
	version      int
	pvalue       float64
	permutations int
}

func (e *eDivisiveMeans) Name() string {
	return "e_divisive_means"
}

func (e *eDivisiveMeans) Version() int {
	return e.version
}

func (e *eDivisiveMeans) Configuration() []AlgorithmConfigurationValue {
	return []AlgorithmConfigurationValue{
		{
			"pvalue",
			e.pvalue,
		},
		{
			"permutations",
			int32(e.permutations),
		},
	}
}

func CreateDefaultAlgorithm() Algorithm {
	return &eDivisiveMeans{
		version:      0,
		pvalue:       0.05,
		permutations: 100,
	}
}

type ChangeDetector interface {
	Algorithm() Algorithm
	DetectChanges(context.Context, []float64) ([]int, error)
}

type jsonChangePoint struct {
	Index     int           `json:"index"`
	Algorithm jsonAlgorithm `json:"algorithm"`
}

type jsonAlgorithm struct {
	Name          string                 `json:"name"`
	Version       int                    `json:"version"`
	Configuration map[string]interface{} `json:"configuration"`
}

type changeDetectionRequest struct {
	Algorithm jsonAlgorithm `json:"algorithm"`
	Series    []float64     `json:"series"`
}

type changeDetectionResponse struct {
	ChangePoints []jsonChangePoint `json:"changePoints"`
}

type signalProcessingClient struct {
	user      string
	token     string
	baseURL   string
	algorithm Algorithm
}

func NewMicroServiceChangeDetector(baseURL, user string, token string, algorithm Algorithm) ChangeDetector {
	return &signalProcessingClient{user: user, token: token, baseURL: baseURL, algorithm: algorithm}
}

func (spc *signalProcessingClient) Algorithm() Algorithm {
	return spc.algorithm
}

func (spc *signalProcessingClient) DetectChanges(ctx context.Context, series []float64) ([]int, error) {
	startAt := time.Now()

	jsonChangePoints := &changeDetectionResponse{}

	if err := spc.doRequest(http.MethodPost, spc.baseURL+"/change_points/detect", ctx, spc.createRequest(series), jsonChangePoints); err != nil {
		return nil, errors.WithStack(err)
	}

	var result []int
	for _, point := range jsonChangePoints.ChangePoints {
		result = append(result, point.Index)
	}

	grip.Debug(message.Fields{
		"message":        "change point detection completed",
		"num_points":     len(series),
		"cp_detected":    len(result),
		"duration_secs":  time.Since(startAt).Seconds(),
		"implementation": "MicroServiceChangePointDetector",
	})

	return result, nil
}

func (spc *signalProcessingClient) createRequest(series []float64) *changeDetectionRequest {
	config := map[string]interface{}{}
	for _, v := range spc.algorithm.Configuration() {
		config[v.Name] = v.Value
	}
	return &changeDetectionRequest{
		Series: series,
		Algorithm: jsonAlgorithm{
			Name:          spc.algorithm.Name(),
			Version:       spc.algorithm.Version(),
			Configuration: config,
		},
	}
}

func (spc *signalProcessingClient) doRequest(method, route string, ctx context.Context, in, out interface{}) error {
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
			"message":   "Failed to detect changes in metric data",
			"status":    http.StatusText(resp.StatusCode),
			"url":       route,
			"auth_user": spc.user,
		})
		return errors.Errorf("Failed to detect changes in metric data, status: %q, url: %q, auth_user: %q", http.StatusText(resp.StatusCode), route, spc.user)
	}

	if err = gimlet.GetJSON(resp.Body, out); err != nil {
		return errors.WithStack(err)
	}

	return nil
}
