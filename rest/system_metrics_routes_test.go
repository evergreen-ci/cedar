package rest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/gimlet"
	"github.com/evergreen-ci/pail"
	"github.com/stretchr/testify/suite"
)

type systemMetricsHandlerSuite struct {
	sc data.MockConnector
	rh map[string]gimlet.RouteHandler

	suite.Suite
}

func (s *systemMetricsHandlerSuite) setup(tempDir string) {
	s.sc = data.MockConnector{
		Bucket: tempDir,
		CachedSystemMetrics: map[string]dbModel.SystemMetrics{
			"abc": dbModel.SystemMetrics{
				ID: "abc",
				Info: dbModel.SystemMetricsInfo{
					Project:   "test",
					Version:   "0",
					Variant:   "linux",
					TaskName:  "task0",
					TaskID:    "task1",
					Execution: 0,
					Mainline:  true,
				},
				Artifact: dbModel.SystemMetricsArtifactInfo{
					Prefix: "abc",
					Options: dbModel.SystemMetricsArtifactOptions{
						Type: dbModel.PailLocal,
					},
				},
			},
			"def": dbModel.SystemMetrics{
				ID: "def",
				Info: dbModel.SystemMetricsInfo{
					Project:   "test",
					Version:   "0",
					Variant:   "linux",
					TaskName:  "task0",
					TaskID:    "task1",
					Execution: 1,
					Mainline:  true,
				},
				Artifact: dbModel.SystemMetricsArtifactInfo{
					Prefix: "def",
					Options: dbModel.SystemMetricsArtifactOptions{
						Type: dbModel.PailLocal,
					},
				},
			},
			"ghi": dbModel.SystemMetrics{
				ID: "ghi",
				Info: dbModel.SystemMetricsInfo{
					Project:   "test",
					Version:   "0",
					Variant:   "linux",
					TaskName:  "task2",
					TaskID:    "task3",
					Execution: 0,
					Mainline:  true,
				},
				Artifact: dbModel.SystemMetricsArtifactInfo{
					Prefix: "ghi",
					Options: dbModel.SystemMetricsArtifactOptions{
						Type: dbModel.PailLocal,
					},
				},
			},
		},
	}

	for key, val := range s.sc.CachedSystemMetrics {
		val.Artifact.MetricChunks = map[string]dbModel.MetricChunks{
			"uptime": dbModel.MetricChunks{
				Chunks: []string{fmt.Sprintf("%v", time.Now())},
				Format: dbModel.FileText,
			},
		}
		s.sc.CachedSystemMetrics[key] = val

		opts := pail.LocalOptions{
			Path:   s.sc.Bucket,
			Prefix: val.Artifact.Prefix,
		}
		bucket, err := pail.NewLocalBucket(opts)
		s.Require().NoError(err)
		data := []byte(fmt.Sprintf("%s-%d", val.Info.TaskID, val.Info.Execution))
		s.Require().NoError(bucket.Put(context.TODO(), val.Artifact.MetricChunks["uptime"].Chunks[0], bytes.NewReader(data)))
	}

	s.rh = map[string]gimlet.RouteHandler{
		"type": makeGetSystemMetricsByType(&s.sc),
	}

}

func TestSystemMetricsHandlerSuite(t *testing.T) {
	s := new(systemMetricsHandlerSuite)
	tempDir, err := ioutil.TempDir(".", "system-metrics")
	s.Require().NoError(err)
	defer func() {
		s.NoError(os.RemoveAll(tempDir))
	}()
	s.setup(tempDir)
	suite.Run(t, s)
}

func (s *systemMetricsHandlerSuite) TestGetSystemMetricsByTypeFound() {
	// no execution
	rh := s.rh["type"]
	rh.(*systemMetricsGetByTypeHandler).opts.TaskID = "task1"
	rh.(*systemMetricsGetByTypeHandler).opts.EmptyExecution = true
	rh.(*systemMetricsGetByTypeHandler).metricType = "uptime"

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	r, ok := resp.Data().(io.ReadCloser)
	s.Require().True(ok)
	data, err := ioutil.ReadAll(r)
	s.NoError(r.Close())
	s.Require().NoError(err)
	s.Equal([]byte("task1-1"), data)

	// execution
	rh.(*systemMetricsGetByTypeHandler).opts.Execution = 0
	rh.(*systemMetricsGetByTypeHandler).opts.EmptyExecution = false

	resp = rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	r, ok = resp.Data().(io.ReadCloser)
	s.Require().True(ok)
	data, err = ioutil.ReadAll(r)
	s.NoError(r.Close())
	s.Require().NoError(err)
	s.Equal([]byte("task1-0"), data)
}

func (s *systemMetricsHandlerSuite) TestGetSystemMetricsByTypeNotFound() {
	// task id DNE
	rh := s.rh["type"]
	rh.(*systemMetricsGetByTypeHandler).opts.TaskID = "DNE"
	rh.(*systemMetricsGetByTypeHandler).opts.Execution = 0
	rh.(*systemMetricsGetByTypeHandler).metricType = "uptime"

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusNotFound, resp.Status())

	// execution DNE
	rh.(*systemMetricsGetByTypeHandler).opts.TaskID = "task1"
	rh.(*systemMetricsGetByTypeHandler).opts.Execution = 5

	resp = rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusNotFound, resp.Status())

	// metric DNE
	rh.(*systemMetricsGetByTypeHandler).opts.TaskID = "task1"
	rh.(*systemMetricsGetByTypeHandler).opts.Execution = 0
	rh.(*systemMetricsGetByTypeHandler).metricType = "DNE"

	resp = rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusNotFound, resp.Status())
}
