package rest

import (
	"context"
	"net/http"
	"net/url"
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
					Tags:     []string{"tag1"},
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
		"id":           makeGetLogByID(&s.sc),
		"meta_id":      makeGetLogMetaByID(&s.sc),
		"task_id":      makeGetLogByTaskID(&s.sc),
		"meta_task_id": makeGetLogMetaByTaskID(&s.sc),
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
	rh := s.rh["meta_id"]
	rh.(*logMetaGetByIDHandler).id = "abc"
	expected := s.apiResults["abc"]

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	s.Require().NotNil(resp.Data())
	s.Equal(&expected, resp.Data())
}

func (s *LogHandlerSuite) TestLogMetaGetByIDHandlerNotFound() {
	rh := s.rh["meta_id"]
	rh.(*logMetaGetByIDHandler).id = "DNE"

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogMetaGetByIDHandlerCtxErr() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rh := s.rh["meta_id"]
	rh.(*logMetaGetByIDHandler).id = "abc"

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogGetByTaskIDHandlerFound() {
	rh := s.rh["task_id"]
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

	// with tags
	rh.(*logGetByTaskIDHandler).tags = []string{"tag1"}
	expected = dbModel.NewLogIteratorReader(
		context.TODO(),
		dbModel.NewMergingIterator(context.TODO(), it1),
	)
	resp = rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	s.Require().NotNil(resp.Data())
	s.Equal(expected, resp.Data())

}

func (s *LogHandlerSuite) TestLogGetByTaskIDHandlerNotFound() {
	rh := s.rh["task_id"]
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
	rh := s.rh["task_id"]
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
	rh := s.rh["meta_task_id"]
	rh.(*logMetaGetByTaskIDHandler).id = "task_id1"
	expected := []model.APILog{s.apiResults["abc"], s.apiResults["def"]}

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	s.Require().NotNil(resp.Data())
	s.Equal(expected, resp.Data())

	// with tags
	rh.(*logMetaGetByTaskIDHandler).tags = []string{"tag1"}
	resp = rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	s.Require().NotNil(resp.Data())
	s.Equal(expected[:1], resp.Data())
}

func (s *LogHandlerSuite) TestLogMetaGetByTaskIDHandlerNotFound() {
	rh := s.rh["meta_task_id"]
	rh.(*logMetaGetByTaskIDHandler).id = "DNE"

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogMetaGetByTaskIDHandlerCtxErr() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rh := s.rh["meta_task_id"]
	rh.(*logMetaGetByTaskIDHandler).id = "task1"

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestParse() {
	for _, test := range []struct {
		urlString string
		handler   string
	}{
		{
			handler:   "id",
			urlString: "http://cedar.mongodb.com/log/id1",
		},
		{
			handler:   "task_id",
			urlString: "http://cedar.mongodb.com/log/task_id/task_id1",
		},
	} {
		s.testParseValid(test.handler, test.urlString)
		s.testParseInvalid(test.handler, test.urlString)
		s.testParseDefaults(test.handler, test.urlString)
	}
}

func (s *LogHandlerSuite) testParseValid(handler, urlString string) {
	ctx := context.Background()
	urlString += "?start=2012-11-01T22:08:00%2B00:00"
	urlString += "&end=2013-11-01T22:08:00%2B00:00"
	urlString += "&limit=5"
	req := &http.Request{Method: "GET"}
	req.URL, _ = url.Parse(urlString)
	expectedTr := util.TimeRange{
		StartAt: time.Date(2012, time.November, 1, 22, 8, 0, 0, time.UTC),
		EndAt:   time.Date(2013, time.November, 1, 22, 8, 0, 0, time.UTC),
	}
	rh := s.rh[handler]

	err := rh.Parse(ctx, req)
	s.Equal(expectedTr, getLogTimeRange(rh, handler))
	s.NoError(err)
}

func (s *LogHandlerSuite) testParseInvalid(handler, urlString string) {
	ctx := context.Background()
	invalidStart := "?start=hello"
	invalidEnd := "?end=world"
	req := &http.Request{Method: "GET"}
	rh := s.rh[handler]

	req.URL, _ = url.Parse(urlString + invalidStart)
	err := rh.Parse(ctx, req)
	s.Error(err)

	req.URL, _ = url.Parse(urlString + invalidEnd)
	err = rh.Parse(ctx, req)
	s.Error(err)
}

func (s *LogHandlerSuite) testParseDefaults(handler, urlString string) {
	ctx := context.Background()
	req := &http.Request{Method: "GET"}
	req.URL, _ = url.Parse(urlString)
	rh := s.rh[handler]

	err := rh.Parse(ctx, req)
	s.Require().NoError(err)
	tr := getLogTimeRange(rh, handler)
	s.Equal(time.Time{}, tr.StartAt)
	s.True(time.Since(tr.EndAt) <= time.Second)
}

func getLogTimeRange(rh gimlet.RouteHandler, handler string) util.TimeRange {
	switch handler {
	case "id":
		return rh.(*logGetByIDHandler).tr
	case "task_id":
		return rh.(*logGetByTaskIDHandler).tr
	default:
		return util.TimeRange{}
	}
}
