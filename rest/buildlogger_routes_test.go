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
	tempDir    string

	suite.Suite
}

func (s *LogHandlerSuite) setup(tempDir string) {
	s.sc = data.MockConnector{
		Bucket: tempDir,
		CachedLogs: map[string]dbModel.Log{
			"abc": {
				ID: "abc",
				Info: dbModel.LogInfo{
					Project:     "project",
					TaskID:      "task_id1",
					TestName:    "test1",
					Execution:   1,
					ProcessName: "mongod0",
					Tags:        []string{"tag1", "group"},
					Mainline:    true,
				},
				CreatedAt:   time.Now().Add(-24 * time.Hour),
				CompletedAt: time.Now().Add(-23 * time.Hour),
				Artifact: dbModel.LogArtifactInfo{
					Type:   dbModel.PailLocal,
					Prefix: "abc",
				},
			},
			"def": {
				ID: "def",
				Info: dbModel.LogInfo{
					Project:  "project",
					TaskID:   "task_id1",
					TestName: "test2",
					Tags:     []string{"tag3", "group"},
					Mainline: true,
				},
				CreatedAt:   time.Now().Add(-25 * time.Hour),
				CompletedAt: time.Now().Add(-23 * time.Hour),
				Artifact: dbModel.LogArtifactInfo{
					Type:   dbModel.PailLocal,
					Prefix: "def",
				},
			},
			"ghi": {
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
			"jkl": {
				ID: "jkl",
				Info: dbModel.LogInfo{
					Project:     "project",
					TaskID:      "task_id1",
					TestName:    "test1",
					ProcessName: "mongod1",
					Tags:        []string{"tag1", "group"},
					Mainline:    true,
				},
				CreatedAt:   time.Now().Add(-26 * time.Hour),
				CompletedAt: time.Now(),
				Artifact: dbModel.LogArtifactInfo{
					Type:   dbModel.PailLocal,
					Prefix: "jkl",
				},
			},
			"mno": {
				ID: "mno",
				Info: dbModel.LogInfo{
					Project:     "project",
					TaskID:      "task_id1",
					ProcessName: "sys",
					Tags:        []string{"tag1", "group"},
					Mainline:    true,
				},
				CreatedAt:   time.Now().Add(-27 * time.Hour),
				CompletedAt: time.Now(),
				Artifact: dbModel.LogArtifactInfo{
					Type:   dbModel.PailLocal,
					Prefix: "mno",
				},
			},
			"pqr": {
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
		"id":              makeGetLogByID(&s.sc),
		"meta_id":         makeGetLogMetaByID(&s.sc),
		"task_id":         makeGetLogByTaskID(&s.sc),
		"meta_task_id":    makeGetLogMetaByTaskID(&s.sc),
		"group_task_id":   makeGetLogGroupByTaskID(&s.sc),
		"test_name":       makeGetLogByTestName(&s.sc),
		"meta_test_name":  makeGetLogMetaByTestName(&s.sc),
		"group_test_name": makeGetLogGroupByTestName(&s.sc),
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
	suite.Run(t, s)
}

func (s *LogHandlerSuite) SetupSuite() {
	tempDir, err := ioutil.TempDir(".", "bucket_test")
	s.Require().NoError(err)
	s.tempDir = tempDir
	s.setup(tempDir)
}

func (s *LogHandlerSuite) TearDownSuite() {
	s.NoError(os.RemoveAll(s.tempDir))
}

func (s *LogHandlerSuite) TestLogGetByIDHandlerFound() {
	for _, printTime := range []bool{true, false} {
		rh := s.rh["id"].Factory()
		rh.(*logGetByIDHandler).opts.ID = "abc"
		rh.(*logGetByIDHandler).opts.TimeRange = dbModel.TimeRange{
			StartAt: time.Now().Add(-24 * time.Hour),
			EndAt:   time.Now(),
		}
		rh.(*logGetByIDHandler).opts.PrintTime = printTime
		rh.(*logGetByIDHandler).opts.PrintPriority = !printTime
		if printTime {
			rh.(*logGetByIDHandler).opts.SoftSizeLimit = softSizeLimit
		}
		it := dbModel.NewBatchedLogIterator(
			s.buckets["abc"],
			s.sc.CachedLogs["abc"].Artifact.Chunks,
			batchSize,
			rh.(*logGetByIDHandler).opts.TimeRange,
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
		if rh.(*logGetByIDHandler).opts.SoftSizeLimit > 0 {
			s.Require().NotNil(pages)
			s.Equal(rh.(*logGetByIDHandler).opts.TimeRange.StartAt.Format(time.RFC3339Nano), pages.Prev.Key)
			s.Nil(pages.Next)
		} else {
			s.Nil(pages)
		}

		// with limit
		rh.(*logGetByIDHandler).opts.Limit = 100
		rh.(*logGetByIDHandler).opts.SoftSizeLimit = 0
		opts.Limit = rh.(*logGetByIDHandler).opts.Limit
		r = dbModel.NewLogIteratorReader(
			context.TODO(),
			dbModel.NewBatchedLogIterator(
				s.buckets["abc"],
				s.sc.CachedLogs["abc"].Artifact.Chunks,
				batchSize,
				rh.(*logGetByIDHandler).opts.TimeRange,
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
	rh := s.rh["id"].Factory()
	rh.(*logGetByIDHandler).opts.ID = "DNE"
	rh.(*logGetByIDHandler).opts.TimeRange = dbModel.TimeRange{
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
	rh := s.rh["id"].Factory()
	rh.(*logGetByIDHandler).opts.ID = "abc"
	rh.(*logGetByIDHandler).opts.TimeRange = dbModel.TimeRange{
		StartAt: time.Now().Add(-24 * time.Hour),
		EndAt:   time.Now(),
	}

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogMetaGetByIDHandlerFound() {
	rh := s.rh["meta_id"].Factory()
	rh.(*logMetaGetByIDHandler).id = "abc"
	expected := s.apiResults["abc"]

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	s.Equal(&expected, resp.Data())
}

func (s *LogHandlerSuite) TestLogMetaGetByIDHandlerNotFound() {
	rh := s.rh["meta_id"].Factory()
	rh.(*logMetaGetByIDHandler).id = "DNE"

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogMetaGetByIDHandlerCtxErr() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rh := s.rh["meta_id"].Factory()
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

		rh := s.rh["task_id"].Factory()
		rh.(*logGetByTaskIDHandler).opts.TaskID = "task_id1"
		rh.(*logGetByTaskIDHandler).opts.TimeRange = dbModel.TimeRange{
			StartAt: time.Now().Add(-24 * time.Hour),
			EndAt:   time.Now(),
		}
		rh.(*logGetByTaskIDHandler).opts.PrintTime = printTime
		rh.(*logGetByTaskIDHandler).opts.PrintPriority = !printTime
		if printTime {
			rh.(*logGetByTaskIDHandler).opts.SoftSizeLimit = softSizeLimit
		}

		it := dbModel.NewMergingIterator(
			dbModel.NewBatchedLogIterator(
				s.buckets["def"],
				s.sc.CachedLogs["def"].Artifact.Chunks,
				batchSize,
				rh.(*logGetByTaskIDHandler).opts.TimeRange,
			),
			dbModel.NewBatchedLogIterator(
				s.buckets["jkl"],
				s.sc.CachedLogs["jkl"].Artifact.Chunks,
				batchSize,
				rh.(*logGetByTaskIDHandler).opts.TimeRange,
			),
			dbModel.NewBatchedLogIterator(
				s.buckets["mno"],
				s.sc.CachedLogs["mno"].Artifact.Chunks,
				batchSize,
				rh.(*logGetByTaskIDHandler).opts.TimeRange,
			),
			dbModel.NewBatchedLogIterator(
				s.buckets["pqr"],
				s.sc.CachedLogs["pqr"].Artifact.Chunks,
				batchSize,
				rh.(*logGetByTaskIDHandler).opts.TimeRange,
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
		if rh.(*logGetByTaskIDHandler).opts.SoftSizeLimit > 0 {
			s.Require().NotNil(pages)
			s.Equal(rh.(*logGetByTaskIDHandler).opts.TimeRange.StartAt.Format(time.RFC3339Nano), pages.Prev.Key)
			s.Nil(pages.Next)
		} else {
			s.Nil(pages)
		}

		// with tags
		rh.(*logGetByTaskIDHandler).opts.Tags = []string{"tag1"}
		it = dbModel.NewMergingIterator(
			dbModel.NewBatchedLogIterator(
				s.buckets["jkl"],
				s.sc.CachedLogs["jkl"].Artifact.Chunks,
				batchSize,
				rh.(*logGetByTaskIDHandler).opts.TimeRange,
			),
			dbModel.NewBatchedLogIterator(
				s.buckets["mno"],
				s.sc.CachedLogs["mno"].Artifact.Chunks,
				batchSize,
				rh.(*logGetByTaskIDHandler).opts.TimeRange,
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
		if rh.(*logGetByTaskIDHandler).opts.SoftSizeLimit > 0 {
			s.Require().NotNil(pages)
			s.Equal(rh.(*logGetByTaskIDHandler).opts.TimeRange.StartAt.Format(time.RFC3339Nano), pages.Prev.Key)
			s.Nil(pages.Next)
		} else {
			s.Nil(pages)
		}

		// with execution
		rh.(*logGetByTaskIDHandler).opts.Execution = 1
		it = dbModel.NewMergingIterator(
			dbModel.NewBatchedLogIterator(
				s.buckets["abc"],
				s.sc.CachedLogs["abc"].Artifact.Chunks,
				batchSize,
				rh.(*logGetByTaskIDHandler).opts.TimeRange,
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
		if rh.(*logGetByTaskIDHandler).opts.SoftSizeLimit > 0 {
			s.Require().NotNil(pages)
			s.Equal(rh.(*logGetByTaskIDHandler).opts.TimeRange.StartAt.Format(time.RFC3339Nano), pages.Prev.Key)
			s.Nil(pages.Next)
		} else {
			s.Nil(pages)
		}

		// with limit
		rh.(*logGetByTaskIDHandler).opts.TaskID = "task_id2"
		rh.(*logGetByTaskIDHandler).opts.Execution = 0
		rh.(*logGetByTaskIDHandler).opts.Tags = []string{}
		rh.(*logGetByTaskIDHandler).opts.Limit = 100
		rh.(*logGetByTaskIDHandler).opts.SoftSizeLimit = 0
		opts.Limit = rh.(*logGetByTaskIDHandler).opts.Limit
		r = dbModel.NewLogIteratorReader(
			context.TODO(),
			dbModel.NewMergingIterator(
				dbModel.NewBatchedLogIterator(
					s.buckets["ghi"],
					s.sc.CachedLogs["ghi"].Artifact.Chunks,
					batchSize,
					rh.(*logGetByTaskIDHandler).opts.TimeRange,
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
		rh.(*logGetByTaskIDHandler).opts.Tail = 100
		opts.TailN = rh.(*logGetByTaskIDHandler).opts.Tail
		r = dbModel.NewLogIteratorReader(
			context.TODO(),
			dbModel.NewMergingIterator(
				dbModel.NewBatchedLogIterator(
					s.buckets["ghi"],
					s.sc.CachedLogs["ghi"].Artifact.Chunks,
					batchSize,
					rh.(*logGetByTaskIDHandler).opts.TimeRange,
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
	rh := s.rh["task_id"].Factory()
	rh.(*logGetByTaskIDHandler).opts.TaskID = "DNE"
	rh.(*logGetByTaskIDHandler).opts.TimeRange = dbModel.TimeRange{
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
	rh := s.rh["task_id"].Factory()
	rh.(*logGetByTaskIDHandler).opts.TaskID = "task_id1"
	rh.(*logGetByTaskIDHandler).opts.TimeRange = dbModel.TimeRange{
		StartAt: time.Now().Add(-24 * time.Hour),
		EndAt:   time.Now(),
	}

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogMetaGetByTaskIDHandlerFound() {
	rh := s.rh["meta_task_id"].Factory()
	rh.(*logMetaGetByTaskIDHandler).opts.TaskID = "task_id1"
	expected := []model.APILog{
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
	rh.(*logMetaGetByTaskIDHandler).opts.Tags = []string{"tag1"}
	resp = rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	s.Equal(expected[1:3], resp.Data())
}

func (s *LogHandlerSuite) TestLogMetaGetByTaskIDHandlerNotFound() {
	rh := s.rh["meta_task_id"].Factory()
	rh.(*logMetaGetByTaskIDHandler).opts.TaskID = "DNE"

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogMetaGetByTaskIDHandlerCtxErr() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rh := s.rh["meta_task_id"].Factory()
	rh.(*logMetaGetByTaskIDHandler).opts.TaskID = "task1"

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogGroupByTaskIDHandlerFound() {
	for _, printTime := range []bool{true, false} {
		opts := dbModel.LogIteratorReaderOptions{
			PrintTime:     printTime,
			PrintPriority: !printTime,
		}

		rh := s.rh["group_task_id"].Factory()
		rh.(*logGroupByTaskIDHandler).opts.TaskID = "task_id1"
		rh.(*logGroupByTaskIDHandler).opts.Group = "group"
		rh.(*logGroupByTaskIDHandler).opts.TimeRange = dbModel.TimeRange{
			StartAt: time.Now().Add(-24 * time.Hour),
			EndAt:   time.Now(),
		}
		rh.(*logGroupByTaskIDHandler).opts.PrintTime = printTime
		rh.(*logGroupByTaskIDHandler).opts.PrintPriority = !printTime
		if printTime {
			rh.(*logGroupByTaskIDHandler).opts.SoftSizeLimit = softSizeLimit
		}

		it := dbModel.NewMergingIterator(
			dbModel.NewBatchedLogIterator(
				s.buckets["def"],
				s.sc.CachedLogs["def"].Artifact.Chunks,
				batchSize,
				rh.(*logGroupByTaskIDHandler).opts.TimeRange,
			),
			dbModel.NewBatchedLogIterator(
				s.buckets["jkl"],
				s.sc.CachedLogs["jkl"].Artifact.Chunks,
				batchSize,
				rh.(*logGroupByTaskIDHandler).opts.TimeRange,
			),
			dbModel.NewBatchedLogIterator(
				s.buckets["mno"],
				s.sc.CachedLogs["mno"].Artifact.Chunks,
				batchSize,
				rh.(*logGroupByTaskIDHandler).opts.TimeRange,
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
		if rh.(*logGroupByTaskIDHandler).opts.SoftSizeLimit > 0 {
			s.Require().NotNil(pages)
			s.Equal(rh.(*logGroupByTaskIDHandler).opts.TimeRange.StartAt.Format(time.RFC3339Nano), pages.Prev.Key)
			s.Nil(pages.Next)
		} else {
			s.Nil(pages)
		}

		// With tags.
		rh.(*logGroupByTaskIDHandler).opts.Tags = []string{"tag1"}
		it = dbModel.NewMergingIterator(
			dbModel.NewBatchedLogIterator(
				s.buckets["jkl"],
				s.sc.CachedLogs["jkl"].Artifact.Chunks,
				batchSize,
				rh.(*logGroupByTaskIDHandler).opts.TimeRange,
			),
			dbModel.NewBatchedLogIterator(
				s.buckets["mno"],
				s.sc.CachedLogs["mno"].Artifact.Chunks,
				batchSize,
				rh.(*logGroupByTaskIDHandler).opts.TimeRange,
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
		if rh.(*logGroupByTaskIDHandler).opts.SoftSizeLimit > 0 {
			s.Require().NotNil(pages)
			s.Equal(rh.(*logGroupByTaskIDHandler).opts.TimeRange.StartAt.Format(time.RFC3339Nano), pages.Prev.Key)
			s.Nil(pages.Next)
		} else {
			s.Nil(pages)
		}

		// With execution.
		rh.(*logGroupByTaskIDHandler).opts.Tags = []string{}
		rh.(*logGroupByTaskIDHandler).opts.Execution = 1
		it = dbModel.NewMergingIterator(
			dbModel.NewBatchedLogIterator(
				s.buckets["abc"],
				s.sc.CachedLogs["abc"].Artifact.Chunks,
				batchSize,
				rh.(*logGroupByTaskIDHandler).opts.TimeRange,
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
		if rh.(*logGroupByTaskIDHandler).opts.SoftSizeLimit > 0 {
			s.Require().NotNil(pages)
			s.Equal(rh.(*logGroupByTaskIDHandler).opts.TimeRange.StartAt.Format(time.RFC3339Nano), pages.Prev.Key)
			s.Nil(pages.Next)
		} else {
			s.Nil(pages)
		}

		// With limit.
		rh.(*logGroupByTaskIDHandler).opts.Execution = 0
		rh.(*logGroupByTaskIDHandler).opts.Limit = 100
		rh.(*logGroupByTaskIDHandler).opts.SoftSizeLimit = 0
		opts.Limit = rh.(*logGroupByTaskIDHandler).opts.Limit
		it = dbModel.NewMergingIterator(
			dbModel.NewBatchedLogIterator(
				s.buckets["def"],
				s.sc.CachedLogs["def"].Artifact.Chunks,
				batchSize,
				rh.(*logGroupByTaskIDHandler).opts.TimeRange,
			),
			dbModel.NewBatchedLogIterator(
				s.buckets["jkl"],
				s.sc.CachedLogs["jkl"].Artifact.Chunks,
				batchSize,
				rh.(*logGroupByTaskIDHandler).opts.TimeRange,
			),
			dbModel.NewBatchedLogIterator(
				s.buckets["mno"],
				s.sc.CachedLogs["mno"].Artifact.Chunks,
				batchSize,
				rh.(*logGroupByTaskIDHandler).opts.TimeRange,
			),
		)
		r = dbModel.NewLogIteratorReader(context.TODO(), it, opts)
		expected, err = ioutil.ReadAll(r)
		s.Require().NoError(err)

		resp = rh.Run(context.TODO())
		s.Require().NotNil(resp)
		s.Equal(http.StatusOK, resp.Status())
		s.Equal(expected, resp.Data())
		s.Nil(resp.Pages())

		resp = rh.Run(context.TODO())
		s.Require().NotNil(resp)
		s.Equal(http.StatusOK, resp.Status())
		s.Equal(expected, resp.Data())
		s.Nil(resp.Pages())
	}
}

func (s *LogHandlerSuite) TestLogGroupByTaskIDHandlerNotFound() {
	rh := s.rh["group_task_id"].Factory()

	// Task ID does not exist.
	rh.(*logGroupByTaskIDHandler).opts.TaskID = "DNE"
	rh.(*logGroupByTaskIDHandler).opts.Group = "group"
	rh.(*logGroupByTaskIDHandler).opts.TimeRange = dbModel.TimeRange{
		StartAt: time.Now().Add(-24 * time.Hour),
		EndAt:   time.Now(),
	}

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())

	// Group does not exist.
	rh.(*logGroupByTaskIDHandler).opts.TaskID = "task_id1"
	rh.(*logGroupByTaskIDHandler).opts.Group = "DNE"

	resp = rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())

}

func (s *LogHandlerSuite) TestLogGroupByTaskIDHandlerCtxErr() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rh := s.rh["group_task_id"].Factory()
	rh.(*logGroupByTaskIDHandler).opts.TaskID = "task_id1"
	rh.(*logGroupByTaskIDHandler).opts.Group = "group"
	rh.(*logGroupByTaskIDHandler).opts.TimeRange = dbModel.TimeRange{
		StartAt: time.Now().Add(-24 * time.Hour),
		EndAt:   time.Now(),
	}

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogGetByTestNameHandlerFound() {
	for _, printTime := range []bool{true, false} {
		rh := s.rh["test_name"].Factory()
		rh.(*logGetByTestNameHandler).opts.TaskID = "task_id1"
		rh.(*logGetByTestNameHandler).opts.TestName = "test1"
		rh.(*logGetByTestNameHandler).opts.Tags = []string{"tag1"}
		rh.(*logGetByTestNameHandler).opts.TimeRange = dbModel.TimeRange{
			StartAt: time.Now().Add(-24 * time.Hour),
			EndAt:   time.Now(),
		}
		rh.(*logGetByTestNameHandler).opts.PrintTime = printTime
		rh.(*logGetByTestNameHandler).opts.PrintPriority = !printTime
		if printTime {
			rh.(*logGetByTestNameHandler).opts.SoftSizeLimit = softSizeLimit
		}
		it := dbModel.NewMergingIterator(
			dbModel.NewBatchedLogIterator(
				s.buckets["jkl"],
				s.sc.CachedLogs["jkl"].Artifact.Chunks,
				batchSize,
				rh.(*logGetByTestNameHandler).opts.TimeRange,
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
		if rh.(*logGetByTestNameHandler).opts.SoftSizeLimit > 0 {
			s.Require().NotNil(pages)
			s.Equal(rh.(*logGetByTestNameHandler).opts.TimeRange.StartAt.Format(time.RFC3339Nano), pages.Prev.Key)
			s.Nil(pages.Next)
		} else {
			s.Nil(pages)
		}

		// limit
		rh.(*logGetByTestNameHandler).opts.Limit = 0
		rh.(*logGetByTestNameHandler).opts.SoftSizeLimit = 0
		r = dbModel.NewLogIteratorReader(
			context.TODO(),
			dbModel.NewMergingIterator(
				dbModel.NewBatchedLogIterator(
					s.buckets["jkl"],
					s.sc.CachedLogs["jkl"].Artifact.Chunks,
					batchSize,
					rh.(*logGetByTestNameHandler).opts.TimeRange,
				),
			),
			dbModel.LogIteratorReaderOptions{
				PrintTime:     printTime,
				PrintPriority: !printTime,
				Limit:         rh.(*logGetByTestNameHandler).opts.Limit,
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
	rh := s.rh["test_name"].Factory()
	rh.(*logGetByTestNameHandler).opts.TaskID = "task_id1"
	rh.(*logGetByTestNameHandler).opts.TestName = "DNE"
	rh.(*logGetByTestNameHandler).opts.Tags = []string{"tag1"}
	rh.(*logGetByTestNameHandler).opts.TimeRange = dbModel.TimeRange{
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
	rh := s.rh["test_name"].Factory()
	rh.(*logGetByTestNameHandler).opts.TaskID = "task_id1"
	rh.(*logGetByTestNameHandler).opts.TestName = "test1"
	rh.(*logGetByTestNameHandler).opts.Tags = []string{"tag1"}
	rh.(*logGetByTestNameHandler).opts.TimeRange = dbModel.TimeRange{
		StartAt: time.Now().Add(-24 * time.Hour),
		EndAt:   time.Now(),
	}

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogMetaGetByTestNameHandlerFound() {
	rh := s.rh["meta_test_name"].Factory()
	rh.(*logMetaGetByTestNameHandler).opts.TaskID = "task_id1"
	rh.(*logMetaGetByTestNameHandler).opts.TestName = "test1"
	rh.(*logMetaGetByTestNameHandler).opts.Tags = []string{"tag1"}
	expected := []model.APILog{
		s.apiResults["jkl"],
		s.apiResults["mno"],
	}

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	s.Equal(expected, resp.Data())

	// only tests
	rh.(*logMetaGetByTestNameHandler).opts.TaskID = "task_id2"
	rh.(*logMetaGetByTestNameHandler).opts.TestName = "test1"
	rh.(*logMetaGetByTestNameHandler).opts.EmptyTestName = false
	rh.(*logMetaGetByTestNameHandler).opts.Tags = []string{}
	expected = []model.APILog{s.apiResults["ghi"]}

	resp = rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	s.Equal(expected, resp.Data())
}

func (s *LogHandlerSuite) TestLogMetaGetByTestNameHandlerNotFound() {
	rh := s.rh["meta_test_name"].Factory()
	rh.(*logMetaGetByTestNameHandler).opts.TaskID = "task_id1"
	rh.(*logMetaGetByTestNameHandler).opts.TestName = "DNE"
	rh.(*logMetaGetByTestNameHandler).opts.Tags = []string{"tag1"}

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogMetaGetByTestNameHandlerCtxErr() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rh := s.rh["meta_test_name"].Factory()
	rh.(*logMetaGetByTestNameHandler).opts.TaskID = "task_id1"
	rh.(*logMetaGetByTestNameHandler).opts.TestName = "test1"
	rh.(*logMetaGetByTestNameHandler).opts.Tags = []string{"tag1"}

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogGroupByTestNameHandlerFound() {
	for _, printTime := range []bool{true, false} {
		opts := dbModel.LogIteratorReaderOptions{
			PrintTime:     printTime,
			PrintPriority: !printTime,
		}

		rh := s.rh["group_test_name"].Factory()
		rh.(*logGroupByTestNameHandler).opts.TaskID = "task_id1"
		rh.(*logGroupByTestNameHandler).opts.TestName = "test1"
		rh.(*logGroupByTestNameHandler).opts.Group = "tag1"
		rh.(*logGroupByTestNameHandler).opts.TimeRange = dbModel.TimeRange{
			StartAt: time.Now().Add(-24 * time.Hour),
			EndAt:   time.Now(),
		}
		rh.(*logGroupByTestNameHandler).opts.PrintTime = printTime
		rh.(*logGroupByTestNameHandler).opts.PrintPriority = !printTime
		rh.(*logGroupByTestNameHandler).opts.Limit = 0
		if printTime {
			rh.(*logGroupByTestNameHandler).opts.SoftSizeLimit = softSizeLimit
		}
		it1 := dbModel.NewBatchedLogIterator(
			s.buckets["jkl"],
			s.sc.CachedLogs["jkl"].Artifact.Chunks,
			batchSize,
			rh.(*logGroupByTestNameHandler).opts.TimeRange,
		)
		it2 := dbModel.NewBatchedLogIterator(
			s.buckets["mno"],
			s.sc.CachedLogs["mno"].Artifact.Chunks,
			batchSize,
			rh.(*logGroupByTestNameHandler).opts.TimeRange,
		)
		it := dbModel.NewMergingIterator(dbModel.NewMergingIterator(it1), dbModel.NewMergingIterator(it2))
		r := dbModel.NewLogIteratorReader(context.TODO(), it, opts)
		expected, err := ioutil.ReadAll(r)
		s.Require().NoError(err)

		resp := rh.Run(context.TODO())
		s.Require().NotNil(resp)
		s.Equal(http.StatusOK, resp.Status())
		s.Equal(expected, resp.Data())
		pages := resp.Pages()
		if rh.(*logGroupByTestNameHandler).opts.SoftSizeLimit > 0 {
			s.Require().NotNil(pages)
			s.Equal(rh.(*logGroupByTestNameHandler).opts.TimeRange.StartAt.Format(time.RFC3339Nano), pages.Prev.Key)
			s.Nil(pages.Next)
		} else {
			s.Nil(pages)
		}

		// limit
		it1 = dbModel.NewBatchedLogIterator(
			s.buckets["jkl"],
			s.sc.CachedLogs["jkl"].Artifact.Chunks,
			batchSize,
			rh.(*logGroupByTestNameHandler).opts.TimeRange,
		)
		it2 = dbModel.NewBatchedLogIterator(
			s.buckets["mno"],
			s.sc.CachedLogs["mno"].Artifact.Chunks,
			batchSize,
			rh.(*logGroupByTestNameHandler).opts.TimeRange,
		)
		rh.(*logGroupByTestNameHandler).opts.Limit = 100
		rh.(*logGroupByTestNameHandler).opts.SoftSizeLimit = 0
		opts.Limit = rh.(*logGroupByTestNameHandler).opts.Limit
		r = dbModel.NewLogIteratorReader(
			context.TODO(),
			dbModel.NewMergingIterator(dbModel.NewMergingIterator(it1), dbModel.NewMergingIterator(it2)),
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
		rh.(*logGroupByTestNameHandler).opts.TestName = "test2"
		rh.(*logGroupByTestNameHandler).opts.Group = "tag3"
		it = dbModel.NewBatchedLogIterator(
			s.buckets["def"],
			s.sc.CachedLogs["def"].Artifact.Chunks,
			batchSize,
			rh.(*logGroupByTestNameHandler).opts.TimeRange,
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

func (s *LogHandlerSuite) TestLogGroupByTestNameHandlerNotFound() {
	rh := s.rh["group_test_name"].Factory()

	// TaskID does not exist.
	rh.(*logGroupByTestNameHandler).opts.TaskID = "DNE"
	rh.(*logGroupByTestNameHandler).opts.TestName = "test1"
	rh.(*logGroupByTestNameHandler).opts.Group = "tag1"
	rh.(*logGroupByTestNameHandler).opts.TimeRange = dbModel.TimeRange{
		StartAt: time.Now().Add(-24 * time.Hour),
		EndAt:   time.Now(),
	}

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())

	// TestName does not exist.
	rh.(*logGroupByTestNameHandler).opts.TaskID = "task_id1"
	rh.(*logGroupByTestNameHandler).opts.TestName = "DNE"
	rh.(*logGroupByTestNameHandler).opts.TimeRange = dbModel.TimeRange{
		StartAt: time.Now().Add(-24 * time.Hour),
		EndAt:   time.Now(),
	}

	resp = rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())

	// Group does not exist.
	rh.(*logGroupByTestNameHandler).opts.TestName = "test1"
	rh.(*logGroupByTestNameHandler).opts.Group = "DNE"
	rh.(*logGroupByTestNameHandler).opts.TimeRange = dbModel.TimeRange{
		StartAt: time.Now().Add(-24 * time.Hour),
		EndAt:   time.Now(),
	}

	resp = rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *LogHandlerSuite) TestLogGroupByTestNameHandlerCtxErr() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rh := s.rh["group_test_name"].Factory()
	rh.(*logGroupByTestNameHandler).opts.TaskID = "task_id1"
	rh.(*logGroupByTestNameHandler).opts.TestName = "test1"
	rh.(*logGroupByTestNameHandler).opts.Group = "tag1"
	rh.(*logGroupByTestNameHandler).opts.TimeRange = dbModel.TimeRange{
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
			handler:   "group_task_id",
			urlString: "http://cedar.mongodb.com/buildlogger/task_id/task_id1/group/group0",
			tags:      true,
		},
		{
			handler:   "test_name",
			urlString: "http://cedar.mongodb.com/buildlogger/test_name/task_id1/test0",
			tags:      true,
		},
		{
			handler:   "group_test_name",
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
	req := &http.Request{Method: "GET"}
	req.URL, _ = url.Parse(urlString)
	expectedTr := dbModel.TimeRange{
		StartAt: time.Date(2012, time.November, 1, 22, 8, 0, 0, time.UTC),
		EndAt:   time.Date(2013, time.November, 1, 22, 8, 0, 0, time.UTC),
	}
	expectedTags := []string{"hello", "world"}
	rh := s.rh[handler].Factory()

	err := rh.Parse(ctx, req)
	s.Require().NoError(err)
	tr, _ := getLogTimeRange(rh, handler)
	s.Equal(expectedTr, tr)
	if tags {
		s.Equal(expectedTags, getLogTags(rh, handler))
	}
	s.True(getLogPrintTime(rh, handler))
	s.True(getLogPrintPriority(rh, handler))
	s.True(getLogPaginate(rh, handler))
	s.Zero(getLogLimit(rh, handler))

	rh = rh.Factory() // need to reset this since we are reusing the handlers
	urlString += "&limit=50"
	req = &http.Request{Method: "GET"}
	req.URL, _ = url.Parse(urlString)
	err = rh.Parse(ctx, req)
	s.Require().NoError(err)
	s.False(getLogPaginate(rh, handler))
	s.Equal(50, getLogLimit(rh, handler))
}

func (s *LogHandlerSuite) testParseInvalid(handler, urlString string) {
	ctx := context.Background()
	invalidStart := "?start=hello"
	invalidEnd := "?end=world"
	req := &http.Request{Method: "GET"}
	rh := s.rh[handler].Factory()

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
	rh := s.rh[handler].Factory()

	err := rh.Parse(ctx, req)
	s.Require().NoError(err)
	tr, expectedDefault := getLogTimeRange(rh, handler)
	s.Equal(expectedDefault.StartAt, tr.StartAt)
	s.True(tr.EndAt.Sub(expectedDefault.EndAt) <= time.Second)
	if tags {
		s.Empty(getLogTags(rh, handler))
	}
	s.False(getLogPrintTime(rh, handler))
	s.False(getLogPaginate(rh, handler))
	s.Zero(getLogLimit(rh, handler))
}

func getLogTimeRange(rh gimlet.RouteHandler, handler string) (dbModel.TimeRange, dbModel.TimeRange) {
	switch handler {
	case "id":
		return rh.(*logGetByIDHandler).opts.TimeRange, dbModel.TimeRange{EndAt: time.Now()}
	case "task_id":
		return rh.(*logGetByTaskIDHandler).opts.TimeRange, dbModel.TimeRange{EndAt: time.Now()}
	case "group_task_id":
		return rh.(*logGroupByTaskIDHandler).opts.TimeRange, dbModel.TimeRange{EndAt: time.Now()}
	case "test_name":
		return rh.(*logGetByTestNameHandler).opts.TimeRange, dbModel.TimeRange{EndAt: time.Now()}
	case "group_test_name":
		return rh.(*logGroupByTestNameHandler).opts.TimeRange, dbModel.TimeRange{}
	default:
		return dbModel.TimeRange{}, dbModel.TimeRange{}
	}
}

func getLogTags(rh gimlet.RouteHandler, handler string) []string {
	switch handler {
	case "task_id":
		return rh.(*logGetByTaskIDHandler).opts.Tags
	case "group_task_id":
		return rh.(*logGroupByTaskIDHandler).opts.Tags
	case "test_name":
		return rh.(*logGetByTestNameHandler).opts.Tags
	case "group_test_name":
		return rh.(*logGroupByTestNameHandler).opts.Tags
	default:
		return []string{}
	}
}

func getLogPrintTime(rh gimlet.RouteHandler, handler string) bool {
	switch handler {
	case "id":
		return rh.(*logGetByIDHandler).opts.PrintTime
	case "task_id":
		return rh.(*logGetByTaskIDHandler).opts.PrintTime
	case "group_task_id":
		return rh.(*logGroupByTaskIDHandler).opts.PrintTime
	case "test_name":
		return rh.(*logGetByTestNameHandler).opts.PrintTime
	case "group_test_name":
		return rh.(*logGroupByTestNameHandler).opts.PrintTime
	default:
		return false
	}
}

func getLogPrintPriority(rh gimlet.RouteHandler, handler string) bool {
	switch handler {
	case "id":
		return rh.(*logGetByIDHandler).opts.PrintPriority
	case "task_id":
		return rh.(*logGetByTaskIDHandler).opts.PrintPriority
	case "group_task_id":
		return rh.(*logGroupByTaskIDHandler).opts.PrintPriority
	case "test_name":
		return rh.(*logGetByTestNameHandler).opts.PrintPriority
	case "group_test_name":
		return rh.(*logGroupByTestNameHandler).opts.PrintPriority
	default:
		return false
	}
}

func getLogPaginate(rh gimlet.RouteHandler, handler string) bool {
	switch handler {
	case "id":
		return rh.(*logGetByIDHandler).opts.SoftSizeLimit > 0
	case "task_id":
		return rh.(*logGetByTaskIDHandler).opts.SoftSizeLimit > 0
	case "group_task_id":
		return rh.(*logGroupByTaskIDHandler).opts.SoftSizeLimit > 0
	case "test_name":
		return rh.(*logGetByTestNameHandler).opts.SoftSizeLimit > 0
	case "group_test_name":
		return rh.(*logGroupByTestNameHandler).opts.SoftSizeLimit > 0
	default:
		return false
	}
}

func getLogLimit(rh gimlet.RouteHandler, handler string) int {
	switch handler {
	case "id":
		return rh.(*logGetByIDHandler).opts.Limit
	case "task_id":
		return rh.(*logGetByTaskIDHandler).opts.Limit
	case "group_task_id":
		return rh.(*logGroupByTaskIDHandler).opts.Limit
	case "test_name":
		return rh.(*logGetByTestNameHandler).opts.Limit
	case "group_test_name":
		return rh.(*logGroupByTestNameHandler).opts.Limit
	default:
		return 0
	}
}

func TestNewBuildloggerResponder(t *testing.T) {
	data := []byte("data")
	last := time.Now().Add(-time.Hour)
	next := time.Now()
	t.Run("NotPaginated", func(t *testing.T) {
		resp := newBuildloggerResponder("base", data, last, next, false)
		assert.Equal(t, data, resp.Data())
		assert.Equal(t, gimlet.TEXT, resp.Format())
		assert.Nil(t, resp.Pages())
	})
	t.Run("PaginatedWithNonZeroNext", func(t *testing.T) {
		resp := newBuildloggerResponder("base", data, last, next, true)
		assert.Equal(t, data, resp.Data())
		assert.Equal(t, gimlet.TEXT, resp.Format())
		pages := resp.Pages()
		require.NotNil(t, pages)
		expectedPrev := &gimlet.Page{
			BaseURL:         "base",
			KeyQueryParam:   "start",
			LimitQueryParam: "limit",
			Key:             last.Format(time.RFC3339Nano),
			Relation:        "prev",
		}
		expectedNext := &gimlet.Page{
			BaseURL:         "base",
			KeyQueryParam:   "start",
			LimitQueryParam: "limit",
			Key:             next.Format(time.RFC3339Nano),
			Relation:        "next",
		}
		assert.Equal(t, expectedPrev, pages.Prev)
		assert.Equal(t, expectedNext, pages.Next)
	})
	t.Run("PaginatedWithZeroNext", func(t *testing.T) {
		resp := newBuildloggerResponder("base", data, last, time.Time{}, true)
		assert.Equal(t, data, resp.Data())
		assert.Equal(t, gimlet.TEXT, resp.Format())
		pages := resp.Pages()
		require.NotNil(t, pages)
		expectedPrev := &gimlet.Page{
			BaseURL:         "base",
			KeyQueryParam:   "start",
			LimitQueryParam: "limit",
			Key:             last.Format(time.RFC3339Nano),
			Relation:        "prev",
		}
		assert.Equal(t, expectedPrev, pages.Prev)
		assert.Nil(t, pages.Next)
	})
}
