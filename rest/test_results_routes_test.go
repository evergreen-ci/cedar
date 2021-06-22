package rest

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/gimlet"
	"github.com/evergreen-ci/pail"
	"github.com/mongodb/amboy/queue"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/suite"
)

const testDBName = "cedar_rest_test"

func newTestEnv() (cedar.Environment, error) {
	env, err := cedar.NewEnvironment(context.Background(), testDBName, &cedar.Configuration{
		MongoDBURI:         "mongodb://localhost:27017",
		DatabaseName:       testDBName,
		SocketTimeout:      time.Minute,
		NumWorkers:         2,
		DisableRemoteQueue: true,
	})
	if err != nil {
		return nil, err
	}
	queue := queue.NewLocalLimitedSize(1, 100)
	err = env.SetRemoteQueue(queue)
	if err != nil {
		return nil, err
	}

	return env, nil
}

func tearDownEnv(env cedar.Environment) error {
	conf, session, err := cedar.GetSessionWithConfig(env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()
	return errors.WithStack(session.DB(conf.DatabaseName).DropDatabase())
}

type TestResultsHandlerSuite struct {
	sc         data.MockConnector
	env        cedar.Environment
	rh         map[string]gimlet.RouteHandler
	apiResults map[string][]model.APITestResult
	buckets    map[string]pail.Bucket

	suite.Suite
}

func (s *TestResultsHandlerSuite) setup(tempDir string) {
	var err error
	s.env, err = newTestEnv()
	s.Require().NoError(err)

	// setup config
	s.Require().NoError(err)
	conf := dbModel.NewCedarConfig(s.env)
	conf.Bucket = dbModel.BucketConfig{TestResultsBucket: tempDir}
	s.Require().NoError(conf.Save())

	s.sc = data.MockConnector{
		Bucket: tempDir,
		CachedTestResults: map[string]dbModel.TestResults{
			"abc": *dbModel.CreateTestResults(
				dbModel.TestResultsInfo{
					Project:       "test",
					Version:       "0",
					Variant:       "linux",
					TaskID:        "task1",
					DisplayTaskID: "display_task1",
					Execution:     0,
					RequestType:   "requesttype",
					Mainline:      true,
				},
				dbModel.PailLocal,
			),
			"def": *dbModel.CreateTestResults(
				dbModel.TestResultsInfo{
					Project:       "test",
					Version:       "0",
					Variant:       "linux",
					TaskID:        "task1",
					DisplayTaskID: "display_task1",
					Execution:     1,
					RequestType:   "requesttype",
					Mainline:      true,
				},
				dbModel.PailLocal,
			),
			"ghi": *dbModel.CreateTestResults(
				dbModel.TestResultsInfo{
					Project:       "test",
					Version:       "0",
					Variant:       "linux",
					TaskID:        "task2",
					DisplayTaskID: "display_task1",
					Execution:     0,
					RequestType:   "requesttype",
					Mainline:      true,
				},
				dbModel.PailLocal,
			),
		},
	}
	s.rh = map[string]gimlet.RouteHandler{
		"task_id":         makeGetTestResultsByTaskID(&s.sc),
		"display_task_id": makeGetTestResultsByDisplayTaskID(&s.sc),
		"test_name":       makeGetTestResultByTestName(&s.sc),
	}
	s.apiResults = map[string][]model.APITestResult{}
	s.buckets = map[string]pail.Bucket{}
	for key, testResults := range s.sc.CachedTestResults {
		var err error
		opts := pail.LocalOptions{
			Path:   s.sc.Bucket,
			Prefix: testResults.Artifact.Prefix,
		}

		s.buckets[key], err = pail.NewLocalBucket(opts)
		s.Require().NoError(err)
		testResults.Setup(s.env)
		s.sc.CachedTestResults[key] = testResults
		for i := 0; i < 3; i++ {
			result := dbModel.TestResult{
				TaskID:    testResults.Info.TaskID,
				Execution: testResults.Info.Execution,
				TestName:  fmt.Sprintf("test%d", i),
				Trial:     0,
				Status:    "teststatus",
				LineNum:   0,
			}
			s.Require().NoError(testResults.Append(context.TODO(), []dbModel.TestResult{result}))

			apiResult := model.APITestResult{}
			s.Require().NoError(apiResult.Import(result))
			s.apiResults[key] = append(s.apiResults[key], apiResult)
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

func (s *TestResultsHandlerSuite) TearDownSuite() {
	err := tearDownEnv(s.env)
	s.Require().NoError(err)
}

func (s *TestResultsHandlerSuite) TestTestResultsGetByTaskIDHandlerFound() {
	rh := s.rh["task_id"]
	optsList := []data.TestResultsOptions{
		{
			TaskID:    "task1",
			Execution: 0,
		},
		{
			TaskID:         "task1",
			EmptyExecution: true,
		},
	}
	expectedResults := [][]model.APITestResult{
		s.apiResults["abc"],
		s.apiResults["def"],
	}

	for i, opts := range optsList {
		rh.(*testResultsGetByTaskIDHandler).opts = opts

		resp := rh.Run(context.TODO())
		s.Require().NotNil(resp)
		s.Equal(http.StatusOK, resp.Status())
		actualResults, ok := resp.Data().([]model.APITestResult)
		s.Require().True(ok)
		s.Equal(expectedResults[i], actualResults)
	}
}

func (s *TestResultsHandlerSuite) TestTestResultsGetByTaskIDHandlerNotFound() {
	rh := s.rh["task_id"]
	rh.(*testResultsGetByTaskIDHandler).opts.TaskID = "DNE"
	rh.(*testResultsGetByTaskIDHandler).opts.Execution = 0

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusNotFound, resp.Status())
}

func (s *TestResultsHandlerSuite) TestTestResultsGetByTaskIDHandlerCtxErr() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rh := s.rh["task_id"]
	rh.(*testResultsGetByTaskIDHandler).opts.TaskID = "task1"
	rh.(*testResultsGetByTaskIDHandler).opts.Execution = 0

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.Equal(http.StatusInternalServerError, resp.Status())
}

func (s *TestResultsHandlerSuite) TestTestResultsGetByDisplayTaskIDHandlerFound() {
	rh := s.rh["display_task_id"]
	optsList := []data.TestResultsOptions{
		{
			DisplayTaskID: "display_task1",
			Execution:     0,
		},
		{
			DisplayTaskID:  "display_task1",
			EmptyExecution: true,
		},
	}
	expectedResults := [][]model.APITestResult{
		append(s.apiResults["abc"], s.apiResults["ghi"]...),
		s.apiResults["def"],
	}

	for i, opts := range optsList {
		rh.(*testResultsGetByDisplayTaskIDHandler).opts = opts

		resp := rh.Run(context.TODO())
		s.Require().NotNil(resp)
		s.Equal(http.StatusOK, resp.Status())
		actualResults, ok := resp.Data().([]model.APITestResult)
		s.Require().True(ok)
		s.Require().Len(actualResults, len(expectedResults[i]))
		for _, actualResult := range actualResults {
			s.Contains(expectedResults[i], actualResult)
		}
	}
}

func (s *TestResultsHandlerSuite) TestTestResultsGetByDisplayTaskIDHandlerNotFound() {
	rh := s.rh["display_task_id"]
	rh.(*testResultsGetByDisplayTaskIDHandler).opts.DisplayTaskID = "DNE"
	rh.(*testResultsGetByDisplayTaskIDHandler).opts.Execution = 0

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusNotFound, resp.Status())
}

func (s *TestResultsHandlerSuite) TestTestResultsGetByDisplayTaskIDHandlerCtxErr() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rh := s.rh["display_task_id"]
	rh.(*testResultsGetByDisplayTaskIDHandler).opts.DisplayTaskID = "display_task1"
	rh.(*testResultsGetByDisplayTaskIDHandler).opts.Execution = 0

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.Equal(http.StatusInternalServerError, resp.Status())
}

func (s *TestResultsHandlerSuite) TestTestResultGetByTestNameHandlerFound() {
	rh := s.rh["test_name"]
	rh.(*testResultGetByTestNameHandler).opts.TaskID = "task1"
	rh.(*testResultGetByTestNameHandler).opts.TestName = "test1"
	rh.(*testResultGetByTestNameHandler).opts.Execution = 0

	expected := s.apiResults["abc"][1]

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
	s.Equal(http.StatusNotFound, resp.Status())
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
	s.Equal(http.StatusInternalServerError, resp.Status())
}

func (s *TestResultsHandlerSuite) TestParse() {
	for _, test := range []struct {
		urlString string
		handler   string
	}{
		{
			handler:   "task_id",
			urlString: "http://cedar.mongodb.com/test_results/task_id/task_id1",
		},
		{
			handler:   "display_task_id",
			urlString: "http://cedar.mongodb.com/test_results/display_task_id/display_task_id1",
		},
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
	// Need to reset this since we are reusing the handlers.
	rh = rh.Factory()

	err := rh.Parse(ctx, req)
	s.Require().NoError(err)
	s.Equal(1, getTestResultsExecution(rh, handler))
}

func (s *TestResultsHandlerSuite) testParseInvalid(handler, urlString string) {
	ctx := context.Background()
	invalidExecution := "?execution=hello"
	req := &http.Request{Method: "GET"}
	rh := s.rh[handler]
	// Need to reset this since we are reusing the handlers.
	rh = rh.Factory()

	req.URL, _ = url.Parse(urlString + invalidExecution)
	err := rh.Parse(ctx, req)
	s.Error(err)
}

func (s *TestResultsHandlerSuite) testParseDefaults(handler, urlString string) {
	ctx := context.Background()
	req := &http.Request{Method: "GET"}
	req.URL, _ = url.Parse(urlString)
	rh := s.rh[handler]
	// Need to reset this since we are reusing the handlers.
	rh = rh.Factory()

	err := rh.Parse(ctx, req)
	s.Require().NoError(err)
	s.True(getTestResultsEmptyExecution(rh, handler))
}

func getTestResultsExecution(rh gimlet.RouteHandler, handler string) int {
	switch handler {
	case "task_id":
		return rh.(*testResultsGetByTaskIDHandler).opts.Execution
	case "display_task_id":
		return rh.(*testResultsGetByDisplayTaskIDHandler).opts.Execution
	case "test_name":
		return rh.(*testResultGetByTestNameHandler).opts.Execution
	default:
		return 0
	}
}

func getTestResultsEmptyExecution(rh gimlet.RouteHandler, handler string) bool {
	switch handler {
	case "task_id":
		return rh.(*testResultsGetByTaskIDHandler).opts.EmptyExecution
	case "display_task_id":
		return rh.(*testResultsGetByDisplayTaskIDHandler).opts.EmptyExecution
	case "test_name":
		return rh.(*testResultGetByTestNameHandler).opts.EmptyExecution
	default:
		return false
	}
}
