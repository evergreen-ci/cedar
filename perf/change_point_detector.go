package perf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/mongodb/grip"

	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/gimlet"
	"github.com/pkg/errors"
)

type ChangeDetector interface {
	DetectChanges([]float64, context.Context) ([]ChangePoint, error)
}

type jsonChangePoint struct {
	Index     int
	Algorithm jsonAlgorithm
}

type jsonAlgorithm struct {
	Name          string
	Version       int
	Configuration map[string]interface{}
}

type ChangePoint struct {
	Index int           `bson:"index" json:"index" yaml:"index"`
	Info  AlgorithmInfo `bson:"info" json:"info" yaml:"info"`
}

type AlgorithmInfo struct {
	Name    string            `bson:"name" json:"name" yaml:"name"`
	Version int               `bson:"version" json:"version" yaml:"version"`
	Options []AlgorithmOption `bson:"options" json:"options" yaml:"options"`
}

type AlgorithmOption struct {
	Name  string      `bson:"name" json:"name" yaml:"name"`
	Value interface{} `bson:"value" json:"value" yaml:"value"`
}

type signalProcessingClient struct {
	user    string
	token   string
	baseURL string
}

func NewMicroServiceChangeDetector(baseURL, user string, token string) ChangeDetector {
	return &signalProcessingClient{user: user, token: token, baseURL: baseURL}
}

func (spc *signalProcessingClient) DetectChanges(series []float64, ctx context.Context) ([]ChangePoint, error) {
	startAt := time.Now()

	jsonChangePoints := &struct {
		ChangePoints []jsonChangePoint `json:"changePoints"`
	}{}

	request := &struct {
		Series []float64 `json:"series"`
	}{
		Series: series,
	}

	if err := spc.doRequest(http.MethodPost, spc.baseURL+"/change_points/detect", ctx, request, jsonChangePoints); err != nil {
		return nil, errors.WithStack(err)
	}

	var result []ChangePoint
	for _, point := range jsonChangePoints.ChangePoints {
		mapped := ChangePoint{
			Index: point.Index,
			Info: AlgorithmInfo{
				Name:    point.Algorithm.Name,
				Version: point.Algorithm.Version,
			},
		}

		for k, v := range point.Algorithm.Configuration {
			additionalOption := AlgorithmOption{
				Name:  k,
				Value: v,
			}
			mapped.Info.Options = append(mapped.Info.Options, additionalOption)
		}

		result = append(result, mapped)
	}

	grip.Debug(map[string]interface{}{
		"message":        "change point detection completed",
		"num_points":     len(series),
		"cp_detected":    len(result),
		"duration_secs":  time.Since(startAt).Seconds(),
		"implementation": "MicroServiceChangePointDetector",
	})

	return result, nil
}

func (spc *signalProcessingClient) doRequest(method, route string, ctx context.Context, in, out interface{}) error {
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
	req.WithContext(ctx)
	req.Header.Add("Cookie", fmt.Sprintf("auth_user=%v;auth_token=%v", spc.user, spc.token))
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return errors.WithStack(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.Errorf("Failed to detect changes in metric data, status: %q", http.StatusText(resp.StatusCode))
	}

	if err = gimlet.GetJSON(resp.Body, out); err != nil {
		return errors.WithStack(err)
	}

	return nil
}