package perf

import (
	"bytes"
	"encoding/json"
	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/gimlet"
	"github.com/pkg/errors"
	"net/http"
)

type ChangeDetector interface {
	DetectChanges([]float64) ([]ChangePoint, error)
}

type ChangePoint struct {
	Index     int
	Algorithm Algorithm
}

type Algorithm struct {
	Name          string
	Version       string
	Configuration map[string]float64
}

type signalProcessingClient struct {
	token   string
	baseURL string
}

func NewChangeDetector(baseURL, token string) ChangeDetector {
	return &signalProcessingClient{token: token, baseURL: baseURL}
}

func (spc *signalProcessingClient) DetectChanges(series []float64) ([]ChangePoint, error) {
	changePoints := &struct {
		ChangePoints []ChangePoint `json:"changePoints"`
	}{}

	if err := spc.doRequest(http.MethodPost, "change_points/detect", series, changePoints); err != nil {
		return nil, errors.WithStack(err)
	}

	return changePoints.ChangePoints, nil
}

func (spc *signalProcessingClient) doRequest(method, route string, in, out interface{}) error {
	body, err := json.Marshal(in)
	if err != nil {
		return errors.WithStack(err)
	}

	client := util.GetHTTPClient()
	defer util.PutHTTPClient(client)

	req, err := http.NewRequest(method, route, bytes.NewBuffer(body))
	if err != nil {
		return errors.WithStack(err)
	}
	req.Header.Add("Authorization", "Bearer "+spc.token)
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return errors.WithStack(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New("Failed to detect changes in metric data, status: "+string(resp.StatusCode))
	}

	if err = gimlet.GetJSON(resp.Body, out); err != nil {
		return errors.WithStack(err)
	}

	return nil
}
