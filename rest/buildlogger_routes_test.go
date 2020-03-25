package rest

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/gimlet"
	"github.com/evergreen-ci/pail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func (s *LogHandlerSuite) setup(tempDir string) {
	s.sc = data.MockConnector{
		Bucket: tempDir,
		CachedLogs: map[string]dbModel.Log{
			"abc": dbModel.Log{
				ID: "abc",
				Info: dbModel.LogInfo{
					Project:     "project",
					TaskID:      "task_id1",
					TestName:    "test1",
					Execution:   1,
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
	}
	s.rh = map[string]gimlet.RouteHandler{
		"id":             makeGetLogByID(&s.sc),
		"meta_id":        makeGetLogMetaByID(&s.sc),
		"task_id":        makeGetLogByTaskID(&s.sc),
		"meta_task_id":   makeGetLogMetaByTaskID(&s.sc),
		"test_name":      makeGetLogByTestName(&s.sc),
		"meta_test_name": makeGetLogMetaByTestName(&s.sc),
		"group":          makeGetLogGroup(&s.sc),
	}
	s.apiResults = map[string]model.APILog{}
	s.buckets = map[string]pail.Bucket{}
	for key, val := range s.sc.CachedLogs {
		var err error
		opts := pail.LocalOptions{
			Path:   s.sc.Bucket,
			Prefix: val.Artifact.Prefix,
		}
		s.buckets[key], err = pail.NewLocalBucket(opts)
		s.Require().NoError(err)
		val.Artifact.Chunks, _, err = dbModel.GenerateTestLog(context.Background(), s.buckets[key], 100, 10)
		s.Require().NoError(err)
		s.sc.CachedLogs[key] = val

		apiResult := model.APILog{}
		s.Require().NoError(apiResult.Import(val))
		s.apiResults[key] = apiResult
	}
}

func TestLogHandlerSuite(t *testing.T) {
	s := new(LogHandlerSuite)
	tempDir, err := ioutil.TempDir(".", "bucket_test")
	s.Require().NoError(err)
	defer func() {
		s.NoError(os.RemoveAll(tempDir))
	}()
	s.setup(tempDir)
	suite.Run(t, s)
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
		rh.(*logGetByIDHandler).printPriority = !printTime
		rh.(*logGetByIDHandler).paginate = printTime
		rh.(*logGetByIDHandler).limit = 0
		it := dbModel.NewBatchedLogIterator(
			s.buckets["abc"],
			s.sc.CachedLogs["abc"].Artifact.Chunks,
			batchSize,
			rh.(*logGetByIDHandler).tr,
		)
		opts := dbModel.LogIteratorReaderOptions{
			PrintTime:     printTime,
			PrintPriority: !printTime,
		}
		r := dbModel.NewLogIteratorReader(context.TODO(), it, opts)
		expected, err := ioutil.ReadAll(r)
		s.Require().NoError(err)

		resp := rh.Run(context.TODO())
		s.Require().NotNil(resp)
		s.Equal(http.StatusOK, resp.Status())
		s.Equal(expected, resp.Data())
		pages := resp.Pages()
		if rh.(*logGetByIDHandler).paginate {
			s.Require().NotNil(pages)
			s.Equal(it.Item().Timestamp.Format(time.RFC3339), pages.Next.Key)
			s.Equal(rh.(*logGetByIDHandler).tr.StartAt.Format(time.RFC3339), pages.Prev.Key)
		} else {
			s.Nil(pages)
		}

		// with limit
		rh.(*logGetByIDHandler).limit = 100
		opts.Limit = rh.(*logGetByIDHandler).limit
		r = dbModel.NewLogIteratorReader(
			context.TODO(),
			dbModel.NewBatchedLogIterator(
				s.buckets["abc"],
				s.sc.CachedLogs["abc"].Artifact.Chunks,
				batchSize,
				rh.(*logGetByIDHandler).tr,
			),
			opts,
		)
		expected, err = ioutil.ReadAll(r)
		s.Require().NoError(err)

		resp = rh.Run(context.TODO())
		s.Require().NotNil(resp)
		s.Equal(http.StatusOK, resp.Status())
		s.Equal(expected, resp.Data())
		s.Nil(resp.Pages())
	}
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
	for _, printTime := range []bool{true, false} {
		opts := dbModel.LogIteratorReaderOptions{
			PrintTime:     printTime,
			PrintPriority: !printTime,
		}

		rh := s.rh["task_id"]
		rh.(*logGetByTaskIDHandler).id = "task_id1"
		rh.(*logGetByTaskIDHandler).tr = util.TimeRange{
			StartAt: time.Now().Add(-24 * time.Hour),
			EndAt:   time.Now(),
		}
		rh.(*logGetByTaskIDHandler).n = 0
		rh.(*logGetByTaskIDHandler).limit = 0
		rh.(*logGetByTaskIDHandler).printTime = printTime
		rh.(*logGetByTaskIDHandler).printPriority = !printTime
		rh.(*logGetByTaskIDHandler).paginate = printTime

		it := dbModel.NewMergingIterator(
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
		)
		r := dbModel.NewLogIteratorReader(context.TODO(), it, opts)
		expected, err := ioutil.ReadAll(r)
		s.Require().NoError(err)

		resp := rh.Run(context.TODO())
		s.Require().NotNil(resp)
		s.Equal(http.StatusOK, resp.Status())
		s.EqualValues(expected, resp.Data())
		pages := resp.Pages()
		if rh.(*logGetByTaskIDHandler).paginate {
			s.Require().NotNil(pages)
			s.Equal(it.Item().Timestamp.Format(time.RFC3339), pages.Next.Key)
			s.Equal(rh.(*logGetByTaskIDHandler).tr.StartAt.Format(time.RFC3339), pages.Prev.Key)
		} else {
			s.Nil(pages)
		}

		// with tags
		rh.(*logGetByTaskIDHandler).tags = []string{"tag1"}
		it = dbModel.NewMergingIterator(
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
		)
		r = dbModel.NewLogIteratorReader(context.TODO(), it, opts)
		expected, err = ioutil.ReadAll(r)
		s.Require().NoError(err)

		resp = rh.Run(context.TODO())
		s.Require().NotNil(resp)
		s.Equal(http.StatusOK, resp.Status())
		s.Equal(expected, resp.Data())
		pages = resp.Pages()
		if rh.(*logGetByTaskIDHandler).paginate {
			s.Require().NotNil(pages)
			s.Equal(it.Item().Timestamp.Format(time.RFC3339), pages.Next.Key)
			s.Equal(rh.(*logGetByTaskIDHandler).tr.StartAt.Format(time.RFC3339), pages.Prev.Key)
		} else {
			s.Nil(pages)
		}

		// with execution
		rh.(*logGetByTaskIDHandler).execution = 1
		it = dbModel.NewMergingIterator(
			dbModel.NewBatchedLogIterator(
				s.buckets["abc"],
				s.sc.CachedLogs["abc"].Artifact.Chunks,
				batchSize,
				rh.(*logGetByTaskIDHandler).tr,
			),
		)
		r = dbModel.NewLogIteratorReader(context.TODO(), it, opts)
		expected, err = ioutil.ReadAll(r)
		s.Require().NoError(err)

		resp = rh.Run(context.TODO())
		s.Require().NotNil(resp)
		s.Equal(http.StatusOK, resp.Status())
		s.Equal(expected, resp.Data())
		pages = resp.Pages()
		if rh.(*logGetByTaskIDHandler).paginate {
			s.Require().NotNil(pages)
			s.Equal(it.Item().Timestamp.Format(time.RFC3339), pages.Next.Key)
			s.Equal(rh.(*logGetByTaskIDHandler).tr.StartAt.Format(time.RFC3339), pages.Prev.Key)
		} else {
			s.Nil(pages)
		}

		// with limit
		rh.(*logGetByTaskIDHandler).id = "task_id2"
		rh.(*logGetByTaskIDHandler).execution = 0
		rh.(*logGetByTaskIDHandler).tags = []string{}
		rh.(*logGetByTaskIDHandler).limit = 100
		opts.Limit = rh.(*logGetByTaskIDHandler).limit
		r = dbModel.NewLogIteratorReader(
			context.TODO(),
			dbModel.NewMergingIterator(
				dbModel.NewBatchedLogIterator(
					s.buckets["ghi"],
					s.sc.CachedLogs["ghi"].Artifact.Chunks,
					batchSize,
					rh.(*logGetByTaskIDHandler).tr,
				),
			),
			opts,
		)
		expected, err = ioutil.ReadAll(r)
		s.Require().NoError(err)

		resp = rh.Run(context.TODO())
		s.Require().NotNil(resp)
		s.Equal(http.StatusOK, resp.Status())
		s.Equal(expected, resp.Data())
		s.Nil(resp.Pages())

		// tail
		rh.(*logGetByTaskIDHandler).n = 100
		opts.TailN = rh.(*logGetByTaskIDHandler).n
		r = dbModel.NewLogIteratorReader(
			context.TODO(),
			dbModel.NewMergingIterator(
				dbModel.NewBatchedLogIterator(
					s.buckets["ghi"],
					s.sc.CachedLogs["ghi"].Artifact.Chunks,
					batchSize,
					rh.(*logGetByTaskIDHandler).tr,
				),
			),
			opts,
		)
		expected, err = ioutil.ReadAll(r)
		s.Require().NoError(err)

		resp = rh.Run(context.TODO())
		s.Require().NotNil(resp)
		s.Equal(http.StatusOK, resp.Status())
		s.Equal(expected, resp.Data())
		s.Nil(resp.Pages())
	}
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
	s.Equal(expected, resp.Data())

	// with tags
	rh.(*logMetaGetByTaskIDHandler).tags = []string{"tag1"}
	resp = rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	s.Equal(append(expected[:1], expected[2:4]...), resp.Data())
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
		rh.(*logGetByTestNameHandler).printPriority = !printTime
		rh.(*logGetByTestNameHandler).paginate = printTime
		rh.(*logGetByTestNameHandler).limit = 0

		it := dbModel.NewMergingIterator(
			dbModel.NewBatchedLogIterator(
				s.buckets["jkl"],
				s.sc.CachedLogs["jkl"].Artifact.Chunks,
				batchSize,
				rh.(*logGetByTestNameHandler).tr,
			),
		)
		r := dbModel.NewLogIteratorReader(
			context.TODO(),
			it,
			dbModel.LogIteratorReaderOptions{
				PrintTime:     printTime,
				PrintPriority: !printTime,
			},
		)
		expected, err := ioutil.ReadAll(r)
		s.Require().NoError(err)

		resp := rh.Run(context.TODO())
		s.Require().NotNil(resp)
		s.Equal(http.StatusOK, resp.Status())
		s.Equal(expected, resp.Data())
		pages := resp.Pages()
		if rh.(*logGetByTestNameHandler).paginate {
			s.Require().NotNil(pages)
			s.Equal(it.Item().Timestamp.Format(time.RFC3339), pages.Next.Key)
			s.Equal(rh.(*logGetByTestNameHandler).tr.StartAt.Format(time.RFC3339), pages.Prev.Key)
		} else {
			s.Nil(pages)
		}

		// limit
		rh.(*logGetByTestNameHandler).limit = 100
		r = dbModel.NewLogIteratorReader(
			context.TODO(),
			dbModel.NewMergingIterator(
				dbModel.NewBatchedLogIterator(
					s.buckets["jkl"],
					s.sc.CachedLogs["jkl"].Artifact.Chunks,
					batchSize,
					rh.(*logGetByTestNameHandler).tr,
				),
			),
			dbModel.LogIteratorReaderOptions{
				PrintTime:     printTime,
				PrintPriority: !printTime,
				Limit:         rh.(*logGetByTestNameHandler).limit,
			},
		)
		expected, err = ioutil.ReadAll(r)
		s.Require().NoError(err)

		resp = rh.Run(context.TODO())
		s.Require().NotNil(resp)
		s.Equal(http.StatusOK, resp.Status())
		s.Equal(expected, resp.Data())
		s.Nil(resp.Pages())
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

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogMetaGetByTestNameHandlerFound() {
	rh := s.rh["meta_test_name"]
	rh.(*logMetaGetByTestNameHandler).id = "task_id1"
	rh.(*logMetaGetByTestNameHandler).name = "test1"
	rh.(*logMetaGetByTestNameHandler).tags = []string{"tag1"}
	expected := []model.APILog{
		s.apiResults["abc"],
		s.apiResults["jkl"],
		s.apiResults["mno"],
	}

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	s.Equal(expected, resp.Data())

	// only tests
	rh.(*logMetaGetByTestNameHandler).name = "test2"
	rh.(*logMetaGetByTestNameHandler).tags = []string{"tag3"}
	expected = []model.APILog{s.apiResults["def"]}

	resp = rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	s.Equal(expected, resp.Data())
}

func (s *LogHandlerSuite) TestLogMetaGetByTestNameHandlerNotFound() {
	rh := s.rh["meta_test_name"]
	rh.(*logMetaGetByTestNameHandler).id = "task_id1"
	rh.(*logMetaGetByTestNameHandler).id = "DNE"
	rh.(*logMetaGetByTestNameHandler).tags = []string{"tag1"}

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

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogGroupHandlerFound() {
	for _, printTime := range []bool{true, false} {
		opts := dbModel.LogIteratorReaderOptions{
			PrintTime:     printTime,
			PrintPriority: !printTime,
		}

		rh := s.rh["group"]
		rh.(*logGroupHandler).id = "task_id1"
		rh.(*logGroupHandler).name = "test1"
		rh.(*logGroupHandler).groupID = "tag1"
		rh.(*logGroupHandler).tr = util.TimeRange{
			StartAt: time.Now().Add(-24 * time.Hour),
			EndAt:   time.Now(),
		}
		rh.(*logGroupHandler).printTime = printTime
		rh.(*logGroupHandler).printPriority = !printTime
		rh.(*logGroupHandler).paginate = printTime
		rh.(*logGroupHandler).limit = 0
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
		it := dbModel.NewMergingIterator(dbModel.NewMergingIterator(it1, it2), dbModel.NewMergingIterator(it3))
		r := dbModel.NewLogIteratorReader(context.TODO(), it, opts)
		expected, err := ioutil.ReadAll(r)
		s.Require().NoError(err)

		resp := rh.Run(context.TODO())
		s.Require().NotNil(resp)
		s.Equal(http.StatusOK, resp.Status())
		s.Equal(expected, resp.Data())
		pages := resp.Pages()
		if rh.(*logGroupHandler).paginate {
			s.Require().NotNil(pages)
			s.Equal(it.Item().Timestamp.Format(time.RFC3339), pages.Next.Key)
			s.Equal(rh.(*logGroupHandler).tr.StartAt.Format(time.RFC3339), pages.Prev.Key)
		} else {
			s.Nil(pages)
		}

		// limit
		it1 = dbModel.NewBatchedLogIterator(
			s.buckets["abc"],
			s.sc.CachedLogs["abc"].Artifact.Chunks,
			batchSize,
			rh.(*logGroupHandler).tr,
		)
		it2 = dbModel.NewBatchedLogIterator(
			s.buckets["jkl"],
			s.sc.CachedLogs["jkl"].Artifact.Chunks,
			batchSize,
			rh.(*logGroupHandler).tr,
		)
		it3 = dbModel.NewBatchedLogIterator(
			s.buckets["mno"],
			s.sc.CachedLogs["mno"].Artifact.Chunks,
			batchSize,
			rh.(*logGroupHandler).tr,
		)
		rh.(*logGroupHandler).limit = 100
		opts.Limit = rh.(*logGroupHandler).limit
		r = dbModel.NewLogIteratorReader(
			context.TODO(),
			dbModel.NewMergingIterator(dbModel.NewMergingIterator(it1, it2), dbModel.NewMergingIterator(it3)),
			opts,
		)
		expected, err = ioutil.ReadAll(r)
		s.Require().NoError(err)

		resp = rh.Run(context.TODO())
		s.Require().NotNil(resp)
		s.Equal(http.StatusOK, resp.Status())
		s.Equal(expected, resp.Data())
		s.Nil(resp.Pages())

		// only tests
		rh.(*logGroupHandler).name = "test2"
		rh.(*logGroupHandler).groupID = "tag3"
		it = dbModel.NewBatchedLogIterator(
			s.buckets["def"],
			s.sc.CachedLogs["def"].Artifact.Chunks,
			batchSize,
			rh.(*logGroupHandler).tr,
		)
		r = dbModel.NewLogIteratorReader(
			context.TODO(),
			dbModel.NewMergingIterator(dbModel.NewMergingIterator(it)),
			opts,
		)
		expected, err = ioutil.ReadAll(r)
		s.Require().NoError(err)

		resp = rh.Run(context.TODO())
		s.Require().NotNil(resp)
		s.Equal(http.StatusOK, resp.Status())
		s.Equal(expected, resp.Data())
		s.Nil(resp.Pages())
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

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestParse() {
	for _, test := range []struct {
		urlString string
		handler   string
		tags      bool
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
		s.testParseInvalid(test.handler, test.urlString)
		s.testParseDefaults(test.handler, test.urlString, test.tags)
	}
}

func (s *LogHandlerSuite) testParseValid(handler, urlString string, tags bool) {
	ctx := context.Background()
	urlString += "?start=2012-11-01T22:08:00%2B00:00"
	urlString += "&end=2013-11-01T22:08:00%2B00:00"
	urlString += "&tags=hello&tags=world"
	urlString += "&print_time=true"
	urlString += "&print_priority=true"
	urlString += "&paginate=true"
	urlString += "&limit=50"
	req := &http.Request{Method: "GET"}
	req.URL, _ = url.Parse(urlString)
	expectedTr := util.TimeRange{
		StartAt: time.Date(2012, time.November, 1, 22, 8, 0, 0, time.UTC),
		EndAt:   time.Date(2013, time.November, 1, 22, 8, 0, 0, time.UTC),
	}
	expectedTags := []string{"hello", "world"}
	rh := s.rh[handler]

	err := rh.Parse(ctx, req)
	tr, _ := getLogTimeRange(rh, handler)
	s.Equal(expectedTr, tr)
	s.NoError(err)
	if tags {
		s.Equal(expectedTags, getLogTags(rh, handler))
	}
	s.True(getLogPrintTime(rh, handler))
	s.True(getLogPrintPriority(rh, handler))
	s.True(getLogPaginate(rh, handler))
	s.Equal(50, getLogLimit(rh, handler))
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
	rh = rh.Factory() // need to reset this since we are reusing the handlers

	err := rh.Parse(ctx, req)
	s.Require().NoError(err)
	tr, expectedDefault := getLogTimeRange(rh, handler)
	s.Equal(expectedDefault.StartAt, tr.StartAt)
	s.True(tr.EndAt.Sub(expectedDefault.EndAt) <= time.Second)
	if tags {
		s.Nil(getLogTags(rh, handler))
	}
	s.False(getLogPrintTime(rh, handler))
	s.False(getLogPaginate(rh, handler))
	s.Zero(getLogLimit(rh, handler))
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

func getLogPrintPriority(rh gimlet.RouteHandler, handler string) bool {
	switch handler {
	case "id":
		return rh.(*logGetByIDHandler).printPriority
	case "task_id":
		return rh.(*logGetByTaskIDHandler).printPriority
	case "test_name":
		return rh.(*logGetByTestNameHandler).printPriority
	case "group":
		return rh.(*logGroupHandler).printPriority
	default:
		return false
	}
}

func getLogPaginate(rh gimlet.RouteHandler, handler string) bool {
	switch handler {
	case "id":
		return rh.(*logGetByIDHandler).paginate
	case "task_id":
		return rh.(*logGetByTaskIDHandler).paginate
	case "test_name":
		return rh.(*logGetByTestNameHandler).paginate
	case "group":
		return rh.(*logGroupHandler).paginate
	default:
		return false
	}
}

func getLogLimit(rh gimlet.RouteHandler, handler string) int {
	switch handler {
	case "id":
		return rh.(*logGetByIDHandler).limit
	case "task_id":
		return rh.(*logGetByTaskIDHandler).limit
	case "test_name":
		return rh.(*logGetByTestNameHandler).limit
	case "group":
		return rh.(*logGroupHandler).limit
	default:
		return 0
	}
}

func TestNewBuildloggerResponder(t *testing.T) {
	data := []byte("data")
	last := time.Now().Add(-time.Hour)
	next := time.Now()
	t.Run("NotPaginated", func(t *testing.T) {
		resp := newBuildloggerResponder(data, last, next, false)
		assert.Equal(t, data, resp.Data())
		assert.Equal(t, gimlet.TEXT, resp.Format())
		assert.Nil(t, resp.Pages())
	})
	t.Run("Paginated", func(t *testing.T) {
		resp := newBuildloggerResponder(data, last, next, true)
		assert.Equal(t, data, resp.Data())
		assert.Equal(t, gimlet.TEXT, resp.Format())
		pages := resp.Pages()
		require.NotNil(t, pages)
		expectedPrev := &gimlet.Page{
			BaseURL:         baseURL,
			KeyQueryParam:   "start",
			LimitQueryParam: "limit",
			Key:             last.Format(time.RFC3339),
			Relation:        "prev",
		}
		expectedNext := &gimlet.Page{
			BaseURL:         baseURL,
			KeyQueryParam:   "start",
			LimitQueryParam: "limit",
			Key:             next.Format(time.RFC3339),
			Relation:        "next",
		}
		assert.Equal(t, expectedPrev, pages.Prev)
		assert.Equal(t, expectedNext, pages.Next)
	})
}
