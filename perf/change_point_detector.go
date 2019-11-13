package perf

import (
	"bytes"
	"encoding/json"
	"github.com/evergreen-ci/cedar"
	"io"
	"net/http"
)

type Detector interface {
	DetectChanges([]float64) ([]ChangePoint, error)
}

type ChangePoint struct {
	Index     int
	Algorithm Algorithm
}

type Algorithm struct {
	Name          string
	Version       string
	Configuration map[string]interface{}
}

type signalProcessingClient struct {
	execute    func(method, url string, body io.Reader) (*http.Response, error)
}


func NewDefaultDetector() Detector {
	return NewDetector(http.DefaultClient.Do, http.NewRequest, cedar.GetEnvironment().GetConf().SignalProcessingURI, cedar.GetEnvironment().GetConf().SignalProcessingToken)
}

func NewDetector(requestExecutor func(req *http.Request) (*http.Response, error), requestMaker func(method, url string, body io.Reader) (*http.Request, error), baseURL string, token string) Detector {
	executor := func(method, url string, body io.Reader) (*http.Response, error) {
		req, err := requestMaker("POST", baseURL+"/change_points/detect", body)
		if err != nil {
			return nil, err
		}
		req.Header.Add("Authorization", "Bearer "+token)
		req.Header.Add("Content-Type", "application/json")
		return requestExecutor(req)
	}

	return signalProcessingClient{
		execute: executor,
	}
}

type detectChangesResponse struct {
	ChangePoints []ChangePoint
}

func (client signalProcessingClient) DetectChanges(series []float64) ([]ChangePoint, error) {
	body, err := json.Marshal(series)
	if err != nil {
		return nil, err
	}
	resp, err := client.execute("POST", "/change_points/detect", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	var decoded detectChangesResponse
	err = json.NewDecoder(resp.Body).Decode(&decoded)
	if err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return decoded.ChangePoints, nil
}
