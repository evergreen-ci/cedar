package rest

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/gimlet"
	"github.com/evergreen-ci/pail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				Chunks: []string{"chunk1", "chunk2"},
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
		chunk1 := []byte(fmt.Sprintf("%s-%d\n", val.Info.TaskID, val.Info.Execution))
		chunk2 := []byte("chunk2")
		s.Require().NoError(bucket.Put(context.TODO(), val.Artifact.MetricChunks["uptime"].Chunks[0], bytes.NewReader(chunk1)))
		s.Require().NoError(bucket.Put(context.TODO(), val.Artifact.MetricChunks["uptime"].Chunks[1], bytes.NewReader(chunk2)))
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
		assert.NoError(t, os.RemoveAll(tempDir))
	}()
	s.setup(tempDir)
	suite.Run(t, s)
}

func (s *systemMetricsHandlerSuite) TestGetSystemMetricsByTypeFound() {
	// no execution
	rh := s.rh["type"]
	rh.(*systemMetricsGetByTypeHandler).findOpts.TaskID = "task1"
	rh.(*systemMetricsGetByTypeHandler).findOpts.EmptyExecution = true
	rh.(*systemMetricsGetByTypeHandler).downloadOpts.MetricType = "uptime"

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	s.Equal([]byte("task1-1\nchunk2"), resp.Data())

	// execution
	rh.(*systemMetricsGetByTypeHandler).findOpts.Execution = 0
	rh.(*systemMetricsGetByTypeHandler).findOpts.EmptyExecution = false

	resp = rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	s.Equal([]byte("task1-0\nchunk2"), resp.Data())

	// pagination
	rh.(*systemMetricsGetByTypeHandler).downloadOpts.PageSize = 5
	resp = rh.Run(context.TODO())
	s.Equal(http.StatusOK, resp.Status())
	s.Equal([]byte("task1-0\n"), resp.Data())

	rh.(*systemMetricsGetByTypeHandler).downloadOpts.StartIndex = 1
	resp = rh.Run(context.TODO())
	s.Equal(http.StatusOK, resp.Status())
	s.Equal([]byte("chunk2"), resp.Data())
}

func (s *systemMetricsHandlerSuite) TestGetSystemMetricsByTypeNotFound() {
	// task id DNE
	rh := s.rh["type"]
	rh.(*systemMetricsGetByTypeHandler).findOpts.TaskID = "DNE"
	rh.(*systemMetricsGetByTypeHandler).findOpts.Execution = 0
	rh.(*systemMetricsGetByTypeHandler).downloadOpts.MetricType = "uptime"

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusNotFound, resp.Status())

	// execution DNE
	rh.(*systemMetricsGetByTypeHandler).findOpts.TaskID = "task1"
	rh.(*systemMetricsGetByTypeHandler).findOpts.Execution = 5

	resp = rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusNotFound, resp.Status())

	// metric DNE
	rh.(*systemMetricsGetByTypeHandler).findOpts.TaskID = "task1"
	rh.(*systemMetricsGetByTypeHandler).findOpts.Execution = 0
	rh.(*systemMetricsGetByTypeHandler).downloadOpts.MetricType = "DNE"

	resp = rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusNotFound, resp.Status())
}

func TestNewSystemMetricsResponder(t *testing.T) {
	data := []byte("data")
	t.Run("PaginatedWithNonZeroNext", func(t *testing.T) {
		resp := newSystemMetricsResponder("base", data, 0, 5)
		assert.Equal(t, data, resp.Data())
		assert.Equal(t, gimlet.TEXT, resp.Format())
		pages := resp.Pages()
		require.NotNil(t, pages)
		expectedPrev := &gimlet.Page{
			BaseURL:         "base",
			KeyQueryParam:   startIndex,
			LimitQueryParam: limit,
			Key:             "0",
			Relation:        "prev",
		}
		expectedNext := &gimlet.Page{
			BaseURL:         "base",
			KeyQueryParam:   startIndex,
			LimitQueryParam: limit,
			Key:             "5",
			Relation:        "next",
		}
		assert.Equal(t, expectedPrev, pages.Prev)
		assert.Equal(t, expectedNext, pages.Next)
	})
	t.Run("PaginatedWithZeroNext", func(t *testing.T) {
		resp := newSystemMetricsResponder("base", data, 5, 5)
		assert.Equal(t, data, resp.Data())
		assert.Equal(t, gimlet.TEXT, resp.Format())
		pages := resp.Pages()
		require.NotNil(t, pages)
		expectedPrev := &gimlet.Page{
			BaseURL:         "base",
			KeyQueryParam:   startIndex,
			LimitQueryParam: limit,
			Key:             "5",
			Relation:        "prev",
		}
		assert.Equal(t, expectedPrev, pages.Prev)
		assert.Nil(t, pages.Next)
	})
}
