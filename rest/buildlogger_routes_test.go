package rest

import (
	"context"
	"net/http"
	"testing"
	"time"

	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/gimlet"
	"github.com/evergreen-ci/pail"
	"github.com/stretchr/testify/suite"
)

const batchSize = 2

type LogHandlerSuite struct {
	sc         data.MockConnector
	rh         map[string]gimlet.RouteHandler
	apiResults map[string]model.APILog
	buckets    map[string]pail.Bucket

	suite.Suite
}

func (s *LogHandlerSuite) setup() {
	s.sc = data.MockConnector{
		Bucket: ".",
		CachedLogs: map[string]dbModel.Log{
			"abc": dbModel.Log{
				ID: "abc",
				Info: dbModel.LogInfo{
					Project:  "project",
					TaskID:   "task_id1",
					TestName: "test1",
					Mainline: true,
				},
				CreatedAt:   time.Now().Add(-24 * time.Hour),
				CompletedAt: time.Now().Add(-23 * time.Hour),
				Artifact: dbModel.LogArtifactInfo{
					Type:   dbModel.PailLocal,
					Prefix: "abc",
				},
			},
			"def": dbModel.Log{
				ID: "def",
				Info: dbModel.LogInfo{
					Project:  "project",
					TaskID:   "task_id1",
					TestName: "test2",
					Mainline: true,
				},
				CreatedAt:   time.Now().Add(-25 * time.Hour),
				CompletedAt: time.Now().Add(-23 * time.Hour),
				Artifact: dbModel.LogArtifactInfo{
					Type:   dbModel.PailLocal,
					Prefix: "def",
				},
			},
			"ghi": dbModel.Log{
				ID: "ghi",
				Info: dbModel.LogInfo{
					Project:  "project",
					TaskID:   "task_id2",
					TestName: "test1",
					Mainline: true,
				},
				CreatedAt:   time.Now().Add(-2 * time.Hour),
				CompletedAt: time.Now(),
				Artifact: dbModel.LogArtifactInfo{
					Type:   dbModel.PailLocal,
					Prefix: "ghi",
				},
			},
		},
	}
	s.rh = map[string]gimlet.RouteHandler{
		"id":         makeGetLogByID(&s.sc),
		"metaID":     makeGetLogMetaByID(&s.sc),
		"taskID":     makeGetLogByTaskID(&s.sc),
		"metaTaskID": makeGetLogMetaByTaskID(&s.sc),
	}
	s.apiResults = map[string]model.APILog{}
	s.buckets = map[string]pail.Bucket{}
	for key, val := range s.sc.CachedLogs {
		apiResult := model.APILog{}
		s.Require().NoError(apiResult.Import(val))
		s.apiResults[key] = apiResult

		var err error
		opts := pail.LocalOptions{
			Path:   s.sc.Bucket,
			Prefix: val.Artifact.Prefix,
		}
		s.buckets[key], err = pail.NewLocalBucket(opts)
		s.Require().NoError(err)
	}
}

func TestLogHandlerSuite(t *testing.T) {
	s := new(LogHandlerSuite)
	s.setup()
	suite.Run(t, s)
}

func (s *LogHandlerSuite) TestLogGetByIDHandlerFound() {
	rh := s.rh["id"]
	rh.(*logGetByIDHandler).id = "abc"
	rh.(*logGetByIDHandler).tr = util.TimeRange{
		StartAt: time.Now().Add(-24 * time.Hour),
		EndAt:   time.Now(),
	}
	it := dbModel.NewBatchedLogIterator(
		s.buckets["abc"],
		s.sc.CachedLogs["abc"].Artifact.Chunks,
		batchSize,
		rh.(*logGetByIDHandler).tr,
	)
	expected := dbModel.NewLogIteratorReader(context.TODO(), it)

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	s.Require().NotNil(resp.Data())
	s.Equal(expected, resp.Data())
}

func (s *LogHandlerSuite) TestLogGetByIDHandlerNotFound() {
	rh := s.rh["id"]
	rh.(*logGetByIDHandler).id = "DNE"
	rh.(*logGetByIDHandler).tr = util.TimeRange{
		StartAt: time.Now().Add(-24 * time.Hour),
		EndAt:   time.Now(),
	}

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogGetByIDHandlerCtxErr() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rh := s.rh["id"]
	rh.(*logGetByIDHandler).id = "abc"
	rh.(*logGetByIDHandler).tr = util.TimeRange{
		StartAt: time.Now().Add(-24 * time.Hour),
		EndAt:   time.Now(),
	}

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogMetaGetByIDHandlerFound() {
	rh := s.rh["metaID"]
	rh.(*logMetaGetByIDHandler).id = "abc"
	expected := s.apiResults["abc"]

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	s.Require().NotNil(resp.Data())
	s.Equal(&expected, resp.Data())
}

func (s *LogHandlerSuite) TestLogMetaGetByIDHandlerNotFound() {
	rh := s.rh["metaID"]
	rh.(*logMetaGetByIDHandler).id = "DNE"

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogMetaGetByIDHandlerCtxErr() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rh := s.rh["metaID"]
	rh.(*logMetaGetByIDHandler).id = "abc"

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogGetByTaskIDHandlerFound() {
	rh := s.rh["taskID"]
	rh.(*logGetByTaskIDHandler).id = "task_id1"
	rh.(*logGetByTaskIDHandler).tr = util.TimeRange{
		StartAt: time.Now().Add(-24 * time.Hour),
		EndAt:   time.Now(),
	}
	it1 := dbModel.NewBatchedLogIterator(
		s.buckets["abc"],
		s.sc.CachedLogs["abc"].Artifact.Chunks,
		batchSize,
		rh.(*logGetByTaskIDHandler).tr,
	)
	it2 := dbModel.NewBatchedLogIterator(
		s.buckets["def"],
		s.sc.CachedLogs["def"].Artifact.Chunks,
		batchSize,
		rh.(*logGetByTaskIDHandler).tr,
	)
	expected := dbModel.NewLogIteratorReader(
		context.TODO(),
		dbModel.NewMergingIterator(context.TODO(), it1, it2),
	)

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	s.Require().NotNil(resp.Data())
	s.Equal(expected, resp.Data())
}

func (s *LogHandlerSuite) TestLogGetByTaskIDHandlerNotFound() {
	rh := s.rh["taskID"]
	rh.(*logGetByTaskIDHandler).id = "DNE"
	rh.(*logGetByTaskIDHandler).tr = util.TimeRange{
		StartAt: time.Now().Add(-24 * time.Hour),
		EndAt:   time.Now(),
	}

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogGetByTaskIDHandlerCtxErr() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rh := s.rh["taskID"]
	rh.(*logGetByTaskIDHandler).id = "task_id1"
	rh.(*logGetByTaskIDHandler).tr = util.TimeRange{
		StartAt: time.Now().Add(-24 * time.Hour),
		EndAt:   time.Now(),
	}

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogMetaGetByTaskIDHandlerFound() {
	rh := s.rh["metaTaskID"]
	rh.(*logMetaGetByTaskIDHandler).id = "task_id1"
	expected := []model.APILog{s.apiResults["abc"], s.apiResults["def"]}

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	s.Require().NotNil(resp.Data())
	s.Equal(expected, resp.Data())
}

func (s *LogHandlerSuite) TestLogMetaGetByTaskIDHandlerNotFound() {
	rh := s.rh["metaTaskID"]
	rh.(*logMetaGetByTaskIDHandler).id = "DNE"

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogMetaGetByTaskIDHandlerCtxErr() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rh := s.rh["metaTaskID"]
	rh.(*logMetaGetByTaskIDHandler).id = "task1"

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}
