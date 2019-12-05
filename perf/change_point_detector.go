package perf

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/mongodb/grip"
	"net/http"

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
	token   string
	baseURL string
}

func NewMicroServiceChangeDetector(baseURL, token string) ChangeDetector {
	return &signalProcessingClient{token: token, baseURL: baseURL}
}

func (spc *signalProcessingClient) DetectChanges(series []float64, ctx context.Context) ([]ChangePoint, error) {
	grip.Debug(`Detecting change points.`)

	jsonChangePoints := &struct {
		ChangePoints []jsonChangePoint `json:"changePoints"`
	}{}

	if err := spc.doRequest(http.MethodPost, spc.baseURL+"/change_points/detect", ctx, series, jsonChangePoints); err != nil {
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

	grip.Debug(`Change point detection completed.`)

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
	req.Header.Add("Authorization", "Bearer "+spc.token)
	req.Header.Add("Content-Type", "application/json")

	grip.Debugf(`Requesting change points from %q.`, route)

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
	grip.Debugf(`Received change points from %q.`, route)

	return nil
}
