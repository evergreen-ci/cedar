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
	evgConf    *dbModel.EvergreenConfig

	suite.Suite
}

func TestLogHandlerSuite(t *testing.T) {
	s := new(LogHandlerSuite)
	suite.Run(t, s)
}

func (s *LogHandlerSuite) SetupSuite() {
	s.sc = data.MockConnector{
		Bucket: ".",
		CachedLogs: map[string]dbModel.Log{
			"abc": dbModel.Log{
				ID: "abc",
				Info: dbModel.LogInfo{
					Project:     "project",
					TaskID:      "task_id1",
					TestName:    "test1",
					ProcessName: "mongod0",
					Tags:        []string{"tag1"},
					Mainline:    true,
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
					Tags:     []string{"tag3"},
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
			"jkl": dbModel.Log{
				ID: "jkl",
				Info: dbModel.LogInfo{
					Project:     "project",
					TaskID:      "task_id1",
					TestName:    "test1",
					ProcessName: "mongod1",
					Tags:        []string{"tag1"},
					Mainline:    true,
				},
				CreatedAt:   time.Now().Add(-26 * time.Hour),
				CompletedAt: time.Now(),
				Artifact: dbModel.LogArtifactInfo{
					Type:   dbModel.PailLocal,
					Prefix: "jkl",
				},
			},
			"mno": dbModel.Log{
				ID: "mno",
				Info: dbModel.LogInfo{
					Project:     "project",
					TaskID:      "task_id1",
					ProcessName: "sys",
					Tags:        []string{"tag1"},
					Mainline:    true,
				},
				CreatedAt:   time.Now().Add(-27 * time.Hour),
				CompletedAt: time.Now(),
				Artifact: dbModel.LogArtifactInfo{
					Type:   dbModel.PailLocal,
					Prefix: "mno",
				},
			},
			"pqr": dbModel.Log{
				ID: "pqr",
				Info: dbModel.LogInfo{
					Project:     "project",
					TaskID:      "task_id1",
					ProcessName: "sys",
					Tags:        []string{"tag2"},
					Mainline:    true,
				},
				CreatedAt:   time.Now().Add(-28 * time.Hour),
				CompletedAt: time.Now(),
				Artifact: dbModel.LogArtifactInfo{
					Type:   dbModel.PailLocal,
					Prefix: "pqr",
				},
			},
		},
		Users: map[string]bool{"user1": true},
	}
	s.evgConf = &dbModel.EvergreenConfig{AuthTokenCookie: "mci-token"}
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

func (s *LogHandlerSuite) SetupTest() {
	s.rh = map[string]gimlet.RouteHandler{
		"id":             makeGetLogByID(&s.sc, s.evgConf),
		"meta_id":        makeGetLogMetaByID(&s.sc, s.evgConf),
		"task_id":        makeGetLogByTaskID(&s.sc, s.evgConf),
		"meta_task_id":   makeGetLogMetaByTaskID(&s.sc, s.evgConf),
		"test_name":      makeGetLogByTestName(&s.sc, s.evgConf),
		"meta_test_name": makeGetLogMetaByTestName(&s.sc, s.evgConf),
		"group":          makeGetLogGroup(&s.sc, s.evgConf),
	}
}

func (s *LogHandlerSuite) TestLogGetByIDHandlerFound() {
	for _, printTime := range []bool{true, false} {
		rh := s.rh["id"]
		rh.(*logGetByIDHandler).id = "abc"
		rh.(*logGetByIDHandler).tr = util.TimeRange{
			StartAt: time.Now().Add(-24 * time.Hour),
			EndAt:   time.Now(),
		}
		rh.(*logGetByIDHandler).printTime = printTime
		rh.(*logGetByIDHandler).userToken = "user1"
		it := dbModel.NewBatchedLogIterator(
			s.buckets["abc"],
			s.sc.CachedLogs["abc"].Artifact.Chunks,
			batchSize,
			rh.(*logGetByIDHandler).tr,
		)
		opts := dbModel.LogIteratorReaderOptions{PrintTime: printTime}
		expected := dbModel.NewLogIteratorReader(context.TODO(), it, opts)

		resp := rh.Run(context.TODO())
		s.Require().NotNil(resp)
		s.Equal(http.StatusOK, resp.Status())
		s.Require().NotNil(resp.Data())
		s.Equal(expected, resp.Data())
	}
}

func (s *LogHandlerSuite) TestLogGetByIDHandlerNotFound() {
	rh := s.rh["id"]
	rh.(*logGetByIDHandler).id = "DNE"
	rh.(*logGetByIDHandler).tr = util.TimeRange{
		StartAt: time.Now().Add(-24 * time.Hour),
		EndAt:   time.Now(),
	}
	rh.(*logGetByIDHandler).userToken = "user1"

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
	rh.(*logGetByIDHandler).userToken = "user1"

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogGetByIDHandlerUnauthedUser() {
	rh := s.rh["id"]
	rh.(*logGetByIDHandler).id = "abc"
	rh.(*logGetByIDHandler).tr = util.TimeRange{
		StartAt: time.Now().Add(-24 * time.Hour),
		EndAt:   time.Now(),
	}
	rh.(*logGetByIDHandler).userToken = "user2"

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusUnauthorized, resp.Status())
}

func (s *LogHandlerSuite) TestLogMetaGetByIDHandlerFound() {
	rh := s.rh["meta_id"]
	rh.(*logMetaGetByIDHandler).id = "abc"
	rh.(*logMetaGetByIDHandler).userToken = "user1"
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
	rh.(*logMetaGetByIDHandler).userToken = "user1"

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogMetaGetByIDHandlerCtxErr() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rh := s.rh["meta_id"]
	rh.(*logMetaGetByIDHandler).id = "abc"
	rh.(*logMetaGetByIDHandler).userToken = "user1"

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogMetaGetByIDHandlerUnauthedUser() {
	rh := s.rh["meta_id"]
	rh.(*logMetaGetByIDHandler).id = "abc"
	rh.(*logMetaGetByIDHandler).userToken = "user2"

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusUnauthorized, resp.Status())
}

func (s *LogHandlerSuite) TestLogGetByTaskIDHandlerFound() {
	for _, printTime := range []bool{true, false} {
		opts := dbModel.LogIteratorReaderOptions{PrintTime: printTime}

		rh := s.rh["task_id"]
		rh.(*logGetByTaskIDHandler).id = "task_id1"
		rh.(*logGetByTaskIDHandler).tr = util.TimeRange{
			StartAt: time.Now().Add(-24 * time.Hour),
			EndAt:   time.Now(),
		}
		rh.(*logGetByTaskIDHandler).n = 0
		rh.(*logGetByTaskIDHandler).printTime = printTime
		rh.(*logGetByTaskIDHandler).userToken = "user1"
		its := []dbModel.LogIterator{
			dbModel.NewBatchedLogIterator(
				s.buckets["abc"],
				s.sc.CachedLogs["abc"].Artifact.Chunks,
				batchSize,
				rh.(*logGetByTaskIDHandler).tr,
			),
			dbModel.NewBatchedLogIterator(
				s.buckets["def"],
				s.sc.CachedLogs["def"].Artifact.Chunks,
				batchSize,
				rh.(*logGetByTaskIDHandler).tr,
			),
			dbModel.NewBatchedLogIterator(
				s.buckets["jkl"],
				s.sc.CachedLogs["jkl"].Artifact.Chunks,
				batchSize,
				rh.(*logGetByTaskIDHandler).tr,
			),
			dbModel.NewBatchedLogIterator(
				s.buckets["mno"],
				s.sc.CachedLogs["mno"].Artifact.Chunks,
				batchSize,
				rh.(*logGetByTaskIDHandler).tr,
			),
			dbModel.NewBatchedLogIterator(
				s.buckets["pqr"],
				s.sc.CachedLogs["pqr"].Artifact.Chunks,
				batchSize,
				rh.(*logGetByTaskIDHandler).tr,
			),
		}
		expected := dbModel.NewLogIteratorReader(
			context.TODO(),
			dbModel.NewMergingIterator(its...),
			opts,
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
			dbModel.NewMergingIterator(append(its[:1], its[2:4]...)...),
			opts,
		)
		resp = rh.Run(context.TODO())
		s.Require().NotNil(resp)
		s.Equal(http.StatusOK, resp.Status())
		s.Require().NotNil(resp.Data())
		s.Equal(expected, resp.Data())

		// tail
		rh.(*logGetByTaskIDHandler).id = "task_id2"
		rh.(*logGetByTaskIDHandler).tags = []string{}
		rh.(*logGetByTaskIDHandler).n = 100
		expectedIt := dbModel.NewMergingIterator(
			dbModel.NewBatchedLogIterator(
				s.buckets["ghi"],
				s.sc.CachedLogs["ghi"].Artifact.Chunks,
				batchSize,
				rh.(*logGetByTaskIDHandler).tr,
			),
		)
		opts.TailN = rh.(*logGetByTaskIDHandler).n
		expected = dbModel.NewLogIteratorReader(
			context.TODO(),
			expectedIt,
			opts,
		)

		resp = rh.Run(context.TODO())
		s.Require().NotNil(resp)
		s.Equal(http.StatusOK, resp.Status())
		s.Require().NotNil(resp.Data())
		s.Equal(expected, resp.Data())
	}
}

func (s *LogHandlerSuite) TestLogGetByTaskIDHandlerNotFound() {
	rh := s.rh["task_id"]
	rh.(*logGetByTaskIDHandler).id = "DNE"
	rh.(*logGetByTaskIDHandler).tr = util.TimeRange{
		StartAt: time.Now().Add(-24 * time.Hour),
		EndAt:   time.Now(),
	}
	rh.(*logGetByTaskIDHandler).userToken = "user1"

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
	rh.(*logGetByTaskIDHandler).userToken = "user1"

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogGetByTaskIDHandlerUnauthedUser() {
	rh := s.rh["task_id"]
	rh.(*logGetByTaskIDHandler).id = "task_id1"
	rh.(*logGetByTaskIDHandler).tr = util.TimeRange{
		StartAt: time.Now().Add(-24 * time.Hour),
		EndAt:   time.Now(),
	}
	rh.(*logGetByTaskIDHandler).userToken = "user2"

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusUnauthorized, resp.Status())
}

func (s *LogHandlerSuite) TestLogMetaGetByTaskIDHandlerFound() {
	rh := s.rh["meta_task_id"]
	rh.(*logMetaGetByTaskIDHandler).id = "task_id1"
	rh.(*logMetaGetByTaskIDHandler).userToken = "user1"
	expected := []model.APILog{
		s.apiResults["abc"],
		s.apiResults["def"],
		s.apiResults["jkl"],
		s.apiResults["mno"],
		s.apiResults["pqr"],
	}

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
	s.Equal(append(expected[:1], expected[2:4]...), resp.Data())
}

func (s *LogHandlerSuite) TestLogMetaGetByTaskIDHandlerNotFound() {
	rh := s.rh["meta_task_id"]
	rh.(*logMetaGetByTaskIDHandler).id = "DNE"
	rh.(*logMetaGetByTaskIDHandler).userToken = "user1"

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogMetaGetByTaskIDHandlerCtxErr() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rh := s.rh["meta_task_id"]
	rh.(*logMetaGetByTaskIDHandler).id = "task1"
	rh.(*logMetaGetByTaskIDHandler).userToken = "user1"

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogMetaGetByTaskIDHandlerUnauthedUser() {
	rh := s.rh["meta_task_id"]
	rh.(*logMetaGetByTaskIDHandler).id = "task_id1"
	rh.(*logMetaGetByTaskIDHandler).userToken = "user2"

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusUnauthorized, resp.Status())
}

func (s *LogHandlerSuite) TestLogGetByTestNameHandlerFound() {
	for _, printTime := range []bool{true, false} {
		rh := s.rh["test_name"]
		rh.(*logGetByTestNameHandler).id = "task_id1"
		rh.(*logGetByTestNameHandler).name = "test1"
		rh.(*logGetByTestNameHandler).tags = []string{"tag1"}
		rh.(*logGetByTestNameHandler).tr = util.TimeRange{
			StartAt: time.Now().Add(-24 * time.Hour),
			EndAt:   time.Now(),
		}
		rh.(*logGetByTestNameHandler).printTime = printTime
		rh.(*logGetByTestNameHandler).userToken = "user1"
		it1 := dbModel.NewBatchedLogIterator(
			s.buckets["abc"],
			s.sc.CachedLogs["abc"].Artifact.Chunks,
			batchSize,
			rh.(*logGetByTestNameHandler).tr,
		)
		it2 := dbModel.NewBatchedLogIterator(
			s.buckets["jkl"],
			s.sc.CachedLogs["jkl"].Artifact.Chunks,
			batchSize,
			rh.(*logGetByTestNameHandler).tr,
		)
		expected := dbModel.NewLogIteratorReader(
			context.TODO(),
			dbModel.NewMergingIterator(it1, it2),
			dbModel.LogIteratorReaderOptions{PrintTime: printTime},
		)

		resp := rh.Run(context.TODO())
		s.Require().NotNil(resp)
		s.Equal(http.StatusOK, resp.Status())
		s.Require().NotNil(resp.Data())
		s.Equal(expected, resp.Data())
	}
}

func (s *LogHandlerSuite) TestLogGetByTestNameHandlerNotFound() {
	rh := s.rh["test_name"]
	rh.(*logGetByTestNameHandler).id = "task_id1"
	rh.(*logGetByTestNameHandler).id = "DNE"
	rh.(*logGetByTestNameHandler).tags = []string{"tag1"}
	rh.(*logGetByTestNameHandler).tr = util.TimeRange{
		StartAt: time.Now().Add(-24 * time.Hour),
		EndAt:   time.Now(),
	}
	rh.(*logGetByTestNameHandler).userToken = "user1"

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogGetByTestNameHandlerCtxErr() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rh := s.rh["test_name"]
	rh.(*logGetByTestNameHandler).id = "task_id1"
	rh.(*logGetByTestNameHandler).name = "test1"
	rh.(*logGetByTestNameHandler).tags = []string{"tag1"}
	rh.(*logGetByTestNameHandler).tr = util.TimeRange{
		StartAt: time.Now().Add(-24 * time.Hour),
		EndAt:   time.Now(),
	}
	rh.(*logGetByTestNameHandler).userToken = "user1"

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogGetByTestNameHandlerUnauthedUser() {
	rh := s.rh["test_name"]
	rh.(*logGetByTestNameHandler).id = "task_id1"
	rh.(*logGetByTestNameHandler).name = "test1"
	rh.(*logGetByTestNameHandler).tags = []string{"tag1"}
	rh.(*logGetByTestNameHandler).tr = util.TimeRange{
		StartAt: time.Now().Add(-24 * time.Hour),
		EndAt:   time.Now(),
	}
	rh.(*logGetByTestNameHandler).userToken = "user2"

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusUnauthorized, resp.Status())
}

func (s *LogHandlerSuite) TestLogMetaGetByTestNameHandlerFound() {
	rh := s.rh["meta_test_name"]
	rh.(*logMetaGetByTestNameHandler).id = "task_id1"
	rh.(*logMetaGetByTestNameHandler).name = "test1"
	rh.(*logMetaGetByTestNameHandler).tags = []string{"tag1"}
	rh.(*logMetaGetByTestNameHandler).userToken = "user1"
	expected := []model.APILog{
		s.apiResults["abc"],
		s.apiResults["jkl"],
		s.apiResults["mno"],
	}

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	s.Require().NotNil(resp.Data())
	s.Equal(expected, resp.Data())

	// only tests
	rh.(*logMetaGetByTestNameHandler).name = "test2"
	rh.(*logMetaGetByTestNameHandler).tags = []string{"tag3"}
	expected = []model.APILog{s.apiResults["def"]}

	resp = rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	s.Require().NotNil(resp.Data())
	s.Equal(expected, resp.Data())
}

func (s *LogHandlerSuite) TestLogMetaGetByTestNameHandlerNotFound() {
	rh := s.rh["meta_test_name"]
	rh.(*logMetaGetByTestNameHandler).id = "task_id1"
	rh.(*logMetaGetByTestNameHandler).id = "DNE"
	rh.(*logMetaGetByTestNameHandler).tags = []string{"tag1"}
	rh.(*logMetaGetByTestNameHandler).userToken = "user1"

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogMetaGetByTestNameHandlerCtxErr() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rh := s.rh["meta_test_name"]
	rh.(*logMetaGetByTestNameHandler).id = "task_id1"
	rh.(*logMetaGetByTestNameHandler).name = "test1"
	rh.(*logMetaGetByTestNameHandler).tags = []string{"tag1"}
	rh.(*logMetaGetByTestNameHandler).userToken = "user1"

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogMetaGetByTestNameHandlerUnauthedUser() {
	rh := s.rh["meta_test_name"]
	rh.(*logMetaGetByTestNameHandler).id = "task_id1"
	rh.(*logMetaGetByTestNameHandler).name = "test1"
	rh.(*logMetaGetByTestNameHandler).tags = []string{"tag1"}
	rh.(*logMetaGetByTestNameHandler).userToken = "user2"

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusUnauthorized, resp.Status())
}

func (s *LogHandlerSuite) TestLogGroupHandlerFound() {
	for _, printTime := range []bool{true, false} {
		opts := dbModel.LogIteratorReaderOptions{PrintTime: printTime}

		rh := s.rh["group"]
		rh.(*logGroupHandler).id = "task_id1"
		rh.(*logGroupHandler).name = "test1"
		rh.(*logGroupHandler).groupID = "tag1"
		rh.(*logGroupHandler).tr = util.TimeRange{
			StartAt: time.Now().Add(-24 * time.Hour),
			EndAt:   time.Now(),
		}
		rh.(*logGroupHandler).printTime = printTime
		rh.(*logGroupHandler).userToken = "user1"
		it1 := dbModel.NewBatchedLogIterator(
			s.buckets["abc"],
			s.sc.CachedLogs["abc"].Artifact.Chunks,
			batchSize,
			rh.(*logGroupHandler).tr,
		)
		it2 := dbModel.NewBatchedLogIterator(
			s.buckets["jkl"],
			s.sc.CachedLogs["jkl"].Artifact.Chunks,
			batchSize,
			rh.(*logGroupHandler).tr,
		)
		it3 := dbModel.NewBatchedLogIterator(
			s.buckets["mno"],
			s.sc.CachedLogs["mno"].Artifact.Chunks,
			batchSize,
			rh.(*logGroupHandler).tr,
		)
		expected := dbModel.NewLogIteratorReader(
			context.TODO(),
			dbModel.NewMergingIterator(dbModel.NewMergingIterator(it1, it2), dbModel.NewMergingIterator(it3)),
			opts,
		)

		resp := rh.Run(context.TODO())
		s.Require().NotNil(resp)
		s.Equal(http.StatusOK, resp.Status())
		s.Require().NotNil(resp.Data())
		s.Equal(expected, resp.Data())

		// only tests
		rh.(*logGroupHandler).name = "test2"
		rh.(*logGroupHandler).groupID = "tag3"
		it := dbModel.NewBatchedLogIterator(
			s.buckets["def"],
			s.sc.CachedLogs["def"].Artifact.Chunks,
			batchSize,
			rh.(*logGroupHandler).tr,
		)
		expected = dbModel.NewLogIteratorReader(
			context.TODO(),
			dbModel.NewMergingIterator(dbModel.NewMergingIterator(it)),
			opts,
		)

		resp = rh.Run(context.TODO())
		s.Require().NotNil(resp)
		s.Equal(http.StatusOK, resp.Status())
		s.Require().NotNil(resp.Data())
		s.Equal(expected, resp.Data())
	}
}

func (s *LogHandlerSuite) TestLogGroupHandlerNotFound() {
	rh := s.rh["group"]
	rh.(*logGroupHandler).id = "task_id1"
	rh.(*logGroupHandler).id = "DNE"
	rh.(*logGroupHandler).groupID = "tag1"
	rh.(*logGroupHandler).tr = util.TimeRange{
		StartAt: time.Now().Add(-24 * time.Hour),
		EndAt:   time.Now(),
	}
	rh.(*logGroupHandler).userToken = "user1"

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogGroupHandlerCtxErr() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rh := s.rh["group"]
	rh.(*logGroupHandler).id = "task_id1"
	rh.(*logGroupHandler).name = "test1"
	rh.(*logGroupHandler).groupID = "tag1"
	rh.(*logGroupHandler).tr = util.TimeRange{
		StartAt: time.Now().Add(-24 * time.Hour),
		EndAt:   time.Now(),
	}
	rh.(*logGroupHandler).userToken = "user1"

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogGroupHandlerUnauthedUser() {
	rh := s.rh["group"]
	rh.(*logGroupHandler).id = "task_id1"
	rh.(*logGroupHandler).name = "test1"
	rh.(*logGroupHandler).groupID = "tag1"
	rh.(*logGroupHandler).tr = util.TimeRange{
		StartAt: time.Now().Add(-24 * time.Hour),
		EndAt:   time.Now(),
	}
	rh.(*logGroupHandler).userToken = "user2"

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusUnauthorized, resp.Status())
}

func (s *LogHandlerSuite) TestParse() {
	for _, test := range []struct {
		urlString string
		handler   string
		tags      bool
		printTime bool
	}{
		{
			handler:   "id",
			urlString: "http://cedar.mongodb.com/buildlogger/id1",
		},
		{
			handler:   "task_id",
			urlString: "http://cedar.mongodb.com/buildlogger/task_id/task_id1",
			tags:      true,
		},
		{
			handler:   "test_name",
			urlString: "http://cedar.mongodb.com/buildlogger/test_name/task_id1/test0",
			tags:      true,
		},
		{
			handler:   "group",
			urlString: "http://cedar.mongodb.com/buildlogger/test_name/task_id1/test0/group/group0",
			tags:      true,
		},
	} {
		s.testParseValid(test.handler, test.urlString, test.tags)
		s.SetupTest()
		s.testParseInvalid(test.handler, test.urlString)
		s.SetupTest()
		s.testParseDefaults(test.handler, test.urlString, test.tags)
	}
}

func (s *LogHandlerSuite) testParseValid(handler, urlString string, tags bool) {
	ctx := context.Background()
	urlString += "?start=2012-11-01T22:08:00%2B00:00"
	urlString += "&end=2013-11-01T22:08:00%2B00:00"
	urlString += "&tags=hello&tags=world"
	urlString += "&printTime=true"
	req, err := http.NewRequest(http.MethodGet, "", nil)
	s.Require().NoError(err)
	req.URL, _ = url.Parse(urlString)
	req.AddCookie(&http.Cookie{Name: "mci-token", Value: "cookie"})
	expectedTr := util.TimeRange{
		StartAt: time.Date(2012, time.November, 1, 22, 8, 0, 0, time.UTC),
		EndAt:   time.Date(2013, time.November, 1, 22, 8, 0, 0, time.UTC),
	}
	expectedTags := []string{"hello", "world"}
	rh := s.rh[handler]

	err = rh.Parse(ctx, req)
	tr, _ := getLogTimeRange(rh, handler)
	s.Equal(expectedTr, tr)
	s.NoError(err)
	if tags {
		s.Equal(expectedTags, getLogTags(rh, handler))
	}
	s.True(getLogPrintTime(rh, handler))
	s.Equal("cookie", getUserToken(rh, handler))
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

func (s *LogHandlerSuite) testParseDefaults(handler, urlString string, tags bool) {
	ctx := context.Background()
	req := &http.Request{Method: "GET"}
	req.URL, _ = url.Parse(urlString)
	rh := s.rh[handler]

	err := rh.Parse(ctx, req)
	s.Require().NoError(err)
	tr, expectedDefault := getLogTimeRange(rh, handler)
	s.Equal(expectedDefault.StartAt, tr.StartAt)
	s.True(tr.EndAt.Sub(expectedDefault.EndAt) <= time.Second)
	if tags {
		s.Nil(getLogTags(rh, handler))
	}
	s.False(getLogPrintTime(rh, handler))
	s.Equal("", getUserToken(rh, handler))
}

func getLogTimeRange(rh gimlet.RouteHandler, handler string) (util.TimeRange, util.TimeRange) {
	switch handler {
	case "id":
		return rh.(*logGetByIDHandler).tr, util.TimeRange{EndAt: time.Now()}
	case "task_id":
		return rh.(*logGetByTaskIDHandler).tr, util.TimeRange{EndAt: time.Now()}
	case "test_name":
		return rh.(*logGetByTestNameHandler).tr, util.TimeRange{EndAt: time.Now()}
	case "group":
		return rh.(*logGroupHandler).tr, util.TimeRange{}
	default:
		return util.TimeRange{}, util.TimeRange{}
	}
}

func getLogTags(rh gimlet.RouteHandler, handler string) []string {
	switch handler {
	case "task_id":
		return rh.(*logGetByTaskIDHandler).tags
	case "test_name":
		return rh.(*logGetByTestNameHandler).tags
	case "group":
		return rh.(*logGroupHandler).tags
	default:
		return []string{}
	}
}

func getLogPrintTime(rh gimlet.RouteHandler, handler string) bool {
	switch handler {
	case "id":
		return rh.(*logGetByIDHandler).printTime
	case "task_id":
		return rh.(*logGetByTaskIDHandler).printTime
	case "test_name":
		return rh.(*logGetByTestNameHandler).printTime
	case "group":
		return rh.(*logGroupHandler).printTime
	default:
		return false
	}
}

func getUserToken(rh gimlet.RouteHandler, handler string) string {
	switch handler {
	case "id":
		return rh.(*logGetByIDHandler).userToken
	case "task_id":
		return rh.(*logGetByTaskIDHandler).userToken
	case "test_name":
		return rh.(*logGetByTestNameHandler).userToken
	case "group":
		return rh.(*logGroupHandler).userToken
	default:
		return ""
	}
}
