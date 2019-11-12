package perf

import (
	"bytes"
	"encoding/json"
	"github.com/evergreen-ci/cedar"
	"net/http"
	"time"
)

type Detector interface {
	DetectChanges([]float64) []ChangePoint
}

type ChangePoint struct {
	Index     int
	Algorithm Algorithm
	CreatedAt time.Time
}

type Algorithm struct {
	Name          string
	Version       int
	Configuration map[string]string
}

type signalProcessingClient struct {
	endpoint string
	token    string
}

func NewDetector() Detector {
	return signalProcessingClient{
		endpoint: cedar.GetEnvironment().GetConf().SignalProcessingURI,
		token:    cedar.GetEnvironment().GetConf().SignalProcessingToken,
	}
}

type detectChangesResponse struct {
	changePoints []ChangePoint
}

func (client signalProcessingClient) DetectChanges(series []float64) []ChangePoint {
	body, _ := json.Marshal(series)
	req, _ := http.NewRequest("POST", client.endpoint+"/change_points/detect", bytes.NewReader(body))
	req.Header.Add("Authorization", "Bearer "+client.token)
	req.Header.Add("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	var response detectChangesResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		panic(err)
	}
	err = resp.Body.Close()
	if err != nil {
		panic(err)
	}
	return response.changePoints
}
