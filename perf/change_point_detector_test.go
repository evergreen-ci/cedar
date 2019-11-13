package perf

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"io"
	"io/ioutil"
	"net/http"
	"testing"
)

func TestDetectChanges(t *testing.T) {
	client := signalProcessingClient{}
	client.execute = func(method, url string, body io.Reader) (*http.Response, error) {
		resp := http.Response{}
		resp.Body = ioutil.NopCloser(bytes.NewBufferString(`
		{
			"changePoints": [
					{
						"index": 12,
						"algorithm": {
							"name": "e_divisive_means",
							"version": "0",
							"configuration": {
								"p_value": 0.05,
								"permutations": 200
							}
						}
					},
					{
						"index": 19,
						"algorithm": {
						"name": "e_divisive_means",
							"version": "0",
							"configuration": {
								"p_value": 0.05,
								"permutations": 200
							}
						}
					}
			]
		}`))
		return &resp, nil
	}
	series := make([]float64, 10)
	resp, _ := client.DetectChanges(series)
	first := resp[0]
	second := resp[1]
	assert.Equal(t, first.Index, 12)
	assert.Equal(t, first.Algorithm.Name, "e_divisive_means")
	assert.Equal(t, first.Algorithm.Version, "0")
	assert.Equal(t, first.Algorithm.Configuration["p_value"], 0.05)
	assert.Equal(t, first.Algorithm.Configuration["permutations"], float64(200))
	assert.Equal(t, second.Index, 19)
	assert.Equal(t, second.Algorithm.Name, "e_divisive_means")
	assert.Equal(t, second.Algorithm.Version, "0")
	assert.Equal(t, second.Algorithm.Configuration["p_value"], 0.05)
	assert.Equal(t, second.Algorithm.Configuration["permutations"], float64(200))
}
