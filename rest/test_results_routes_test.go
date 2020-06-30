package rest

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/gimlet"
	"github.com/evergreen-ci/pail"
	"github.com/stretchr/testify/suite"
	"gopkg.in/mgo.v2/bson"
)

type TestResultsHandlerSuite struct {
	sc         data.MockConnector
	rh         map[string]gimlet.RouteHandler
	apiResults map[string]model.APITestResult
	buckets    map[string]pail.Bucket

	suite.Suite
}

func (s *TestResultsHandlerSuite) setup(tempDir string) {
	s.sc = data.MockConnector{
		Bucket: tempDir,
		CachedTestResults: map[string]dbModel.TestResults{
			"abc": dbModel.TestResults{
				ID: "abc",
				Info: dbModel.TestResultsInfo{
					Project:     "test",
					Version:     "0",
					Variant:     "linux",
					TaskName:    "task0",
					TaskID:      "task1",
					Execution:   0,
					RequestType: "requesttype",
					Mainline:    true,
				},
				CreatedAt:   time.Now().Add(-24 * time.Hour),
				CompletedAt: time.Now().Add(-23 * time.Hour),
				Artifact: dbModel.TestResultsArtifactInfo{
					Type:   dbModel.PailLocal,
					Prefix: "abc",
				},
			},
			"def": dbModel.TestResults{
				ID: "def",
				Info: dbModel.TestResultsInfo{
					Project:     "test",
					Version:     "0",
					Variant:     "linux",
					TaskName:    "task0",
					TaskID:      "task1",
					Execution:   1,
					RequestType: "requesttype",
					Mainline:    true,
				},
				CreatedAt:   time.Now().Add(-25 * time.Hour),
				CompletedAt: time.Now().Add(-23 * time.Hour),
				Artifact: dbModel.TestResultsArtifactInfo{
					Type:   dbModel.PailLocal,
					Prefix: "def",
				},
			},
			"ghi": dbModel.TestResults{
				ID: "ghi",
				Info: dbModel.TestResultsInfo{
					Project:     "test",
					Version:     "0",
					Variant:     "linux",
					TaskName:    "task2",
					TaskID:      "task3",
					Execution:   0,
					RequestType: "requesttype",
					Mainline:    true,
				},
				CreatedAt:   time.Now().Add(-2 * time.Hour),
				CompletedAt: time.Now(),
				Artifact: dbModel.TestResultsArtifactInfo{
					Type:   dbModel.PailLocal,
					Prefix: "ghi",
				},
			},
		},
	}
	s.rh = map[string]gimlet.RouteHandler{
		"test_name": makeGetTestResultByTestName(&s.sc),
	}
	s.apiResults = map[string]model.APITestResult{}
	s.buckets = map[string]pail.Bucket{}
	for key, testResults := range s.sc.CachedTestResults {
		var err error
		opts := pail.LocalOptions{
			Path:   s.sc.Bucket,
			Prefix: filepath.Join("test_results", testResults.Artifact.Prefix),
		}

		s.buckets[key], err = pail.NewLocalBucket(opts)
		s.Require().NoError(err)
		s.sc.CachedTestResults[key] = testResults
		for i := 0; i < 3; i++ {
			result := dbModel.TestResult{
				TaskID:         testResults.Info.TaskID,
				Execution:      testResults.Info.Execution,
				TestName:       fmt.Sprintf("test%d", i),
				Trial:          0,
				Status:         "teststatus",
				LogURL:         "testurl",
				LineNum:        0,
				TaskCreateTime: time.Now().Add(-3 * time.Second),
				TestStartTime:  time.Now().Add(-2 * time.Second),
				TestEndTime:    time.Now().Add(-1 * time.Second),
			}

			data, err := bson.Marshal(result)
			s.Require().NoError(err)
			s.Require().NoError(s.buckets[key].Put(context.Background(), result.TestName, bytes.NewReader(data)))
			apiResult := model.APITestResult{}
			s.Require().NoError(apiResult.Import(result))
			s.apiResults[fmt.Sprintf("%s%d%s", result.TaskID, result.Execution, result.TestName)] = apiResult
		}
	}
}

func TestTestResultsHandlerSuite(t *testing.T) {
	s := new(TestResultsHandlerSuite)
	tempDir, err := ioutil.TempDir(".", "bucket_test")
	s.Require().NoError(err)
	defer func() {
		s.NoError(os.RemoveAll(tempDir))
	}()
	s.setup(tempDir)
	suite.Run(t, s)
}

func (s *TestResultsHandlerSuite) TestTestResultGetByTaskIdHandlerFound() {
	rh := s.rh["test_name"]
	rh.(*testResultGetByTestNameHandler).opts.TaskID = "task1"
	rh.(*testResultGetByTestNameHandler).opts.TestName = "test1"
	rh.(*testResultGetByTestNameHandler).opts.Execution = 0

	expected, ok := s.apiResults["task10test1"] // retrieve stored apiTestResult
	s.True(ok)

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	actual, ok := resp.Data().(*model.APITestResult)
	s.Require().True(ok)
	s.Equal(expected.TestName, actual.TestName)
	s.Equal(expected.TaskID, actual.TaskID)
	s.Equal(expected.Execution, actual.Execution)
}

func (s *TestResultsHandlerSuite) TestTestResultGetByTestNameHandlerFound() {
	rh := s.rh["test_name"]
	rh.(*testResultGetByTestNameHandler).opts.TaskID = "task1"
	rh.(*testResultGetByTestNameHandler).opts.TestName = "test1"
	rh.(*testResultGetByTestNameHandler).opts.Execution = 0

	expected, ok := s.apiResults["task10test1"] // retrieve stored apiTestResult
	s.True(ok)

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	actual, ok := resp.Data().(*model.APITestResult)
	s.Require().True(ok)
	s.Equal(expected.TestName, actual.TestName)
	s.Equal(expected.TaskID, actual.TaskID)
	s.Equal(expected.Execution, actual.Execution)
}

func (s *TestResultsHandlerSuite) TestTestResultGetByTestNameHandlerNotFound() {
	rh := s.rh["test_name"]
	rh.(*testResultGetByTestNameHandler).opts.TaskID = "task1"
	rh.(*testResultGetByTestNameHandler).opts.TestName = "DNE"
	rh.(*testResultGetByTestNameHandler).opts.Execution = 0

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *TestResultsHandlerSuite) TestTestResultGetByTestNameHandlerCtxErr() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rh := s.rh["test_name"]
	rh.(*testResultGetByTestNameHandler).opts.TaskID = "task1"
	rh.(*testResultGetByTestNameHandler).opts.TestName = "test1"
	rh.(*testResultGetByTestNameHandler).opts.Execution = 0

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *TestResultsHandlerSuite) TestParse() {
	for _, test := range []struct {
		urlString string
		handler   string
	}{
		{
			handler:   "test_name",
			urlString: "http://cedar.mongodb.com/test_results/test_name/task_id1/test0",
		},
	} {
		s.testParseValid(test.handler, test.urlString)
		s.testParseInvalid(test.handler, test.urlString)
		s.testParseDefaults(test.handler, test.urlString)
	}
}

func (s *TestResultsHandlerSuite) testParseValid(handler, urlString string) {
	ctx := context.Background()
	urlString += "?execution=1"
	req := &http.Request{Method: "GET"}
	req.URL, _ = url.Parse(urlString)
	rh := s.rh[handler]
	rh = rh.Factory() // need to reset this since we are reusing the handlers

	err := rh.Parse(ctx, req)
	s.Require().NoError(err)
	s.True(getTestResultsExecution(rh, handler))
}

func (s *TestResultsHandlerSuite) testParseInvalid(handler, urlString string) {
	ctx := context.Background()
	invalidExecution := "?execution=hello"
	req := &http.Request{Method: "GET"}
	rh := s.rh[handler]
	rh = rh.Factory() // need to reset this since we are reusing the handlers

	req.URL, _ = url.Parse(urlString + invalidExecution)
	err := rh.Parse(ctx, req)
	s.Error(err)
}

func (s *TestResultsHandlerSuite) testParseDefaults(handler, urlString string) {
	ctx := context.Background()
	req := &http.Request{Method: "GET"}
	req.URL, _ = url.Parse(urlString)
	rh := s.rh[handler]
	rh = rh.Factory() // need to reset this since we are reusing the handlers

	err := rh.Parse(ctx, req)
	s.Require().NoError(err)
	s.False(getTestResultsExecution(rh, handler))
}

func getTestResultsExecution(rh gimlet.RouteHandler, handler string) bool {
	switch handler {
	case "test_name":
		return !rh.(*testResultGetByTestNameHandler).opts.EmptyExecution
	default:
		return false
	}
}
