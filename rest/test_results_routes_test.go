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
	"github.com/evergreen-ci/utility"
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
	sc         data.Connector
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

	s.Require().NoError(err)
	conf := dbModel.NewCedarConfig(s.env)
	conf.Bucket = dbModel.BucketConfig{TestResultsBucket: tempDir}
	s.Require().NoError(conf.Save())

	s.sc = data.CreateNewDBConnector(s.env, "")
	s.apiResults = map[string][]model.APITestResult{}
	for key, testResults := range map[string]dbModel.TestResults{
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
	} {
		s.Require().NoError(err)
		testResults.Setup(s.env)
		s.Require().NoError(testResults.SaveNew(context.TODO()))
		for i := 0; i < 3; i++ {
			result := dbModel.TestResult{
				TaskID:    testResults.Info.TaskID,
				Execution: testResults.Info.Execution,
				TestName:  fmt.Sprintf("test%d", i),
				Trial:     0,
				Status:    "teststatus-fail",
				LineNum:   0,
			}
			s.Require().NoError(testResults.Append(context.TODO(), []dbModel.TestResult{result}))

			apiResult := model.APITestResult{}
			s.Require().NoError(apiResult.Import(result))
			s.apiResults[key] = append(s.apiResults[key], apiResult)
		}
	}

	s.rh = map[string]gimlet.RouteHandler{
		"task_id":             makeGetTestResultsByTaskID(s.sc),
		"failed_tests_sample": makeGetTestResultsFailedSample(s.sc),
		"stats":               makeGetTestResultsStats(s.sc),
		"display_task_id":     makeGetTestResultsByDisplayTaskID(s.sc),
		"test_name":           makeGetTestResultByTestName(s.sc),
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

func (s *TestResultsHandlerSuite) TestTestResultsGetByTaskIDHandler() {
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	rh := s.rh["task_id"].(*testResultsGetByTaskIDHandler)

	for _, test := range []struct {
		name            string
		ctx             context.Context
		opts            data.TestResultsOptions
		expectedResults []model.APITestResult
		errorStatus     int
	}{
		{
			name:        "FailsWithContextError",
			ctx:         canceledCtx,
			opts:        data.TestResultsOptions{TaskID: "task1"},
			errorStatus: http.StatusInternalServerError,
		},
		{
			name:        "FailsWhenTaskIDDNE",
			opts:        data.TestResultsOptions{TaskID: "DNE"},
			errorStatus: http.StatusNotFound,
		},
		{
			name:        "FailsWhenDisplayTaskIDDNE",
			opts:        data.TestResultsOptions{TaskID: "DNE", DisplayTask: true},
			errorStatus: http.StatusNotFound,
		},
		{
			name: "SucceedsWithTaskIDAndExecution",
			opts: data.TestResultsOptions{
				TaskID:    "task1",
				Execution: utility.ToIntPtr(0),
			},
			expectedResults: s.apiResults["abc"],
		},
		{
			name:            "TaskIDWithoutExecution",
			opts:            data.TestResultsOptions{TaskID: "task1"},
			expectedResults: s.apiResults["def"],
		},
		{
			name: "SucceedsWithTaskIDAndFilterAndSort",
			opts: data.TestResultsOptions{
				TaskID:    "task1",
				Execution: utility.ToIntPtr(0),
				FilterAndSort: &data.TestResultsFilterAndSortOptions{
					TestName: "test1",
				},
			},
			expectedResults: s.apiResults["abc"][1:2],
		},
		{
			name: "SucceedsWithDisplayTaskIDAndExecution",
			opts: data.TestResultsOptions{
				TaskID:      "display_task1",
				Execution:   utility.ToIntPtr(1),
				DisplayTask: true,
			},
			expectedResults: s.apiResults["def"],
		},
		{
			name: "SucceedsWithDisplayTaskIDAndNoExecution",
			opts: data.TestResultsOptions{
				TaskID:      "display_task1",
				DisplayTask: true,
			},
			expectedResults: s.apiResults["def"],
		},
		{
			name: "SucceedsWithDisplayTaskIDAndFilterAndSort",
			opts: data.TestResultsOptions{
				TaskID:      "display_task1",
				DisplayTask: true,
				FilterAndSort: &data.TestResultsFilterAndSortOptions{
					TestName: "test1",
				},
			},
			expectedResults: s.apiResults["def"][1:2],
		},
	} {
		s.T().Run(test.name, func(t *testing.T) {
			rh.opts = test.opts
			ctx := test.ctx
			if ctx == nil {
				ctx = context.TODO()
			}
			resp := rh.Run(ctx)

			s.Require().NotNil(resp)
			if test.errorStatus > 0 {
				s.Equal(test.errorStatus, resp.Status())
			} else {
				s.Equal(http.StatusOK, resp.Status())
				actualResults, ok := resp.Data().([]model.APITestResult)
				s.Require().True(ok)
				s.Equal(test.expectedResults, actualResults)
			}
		})
	}
}

func (s *TestResultsHandlerSuite) TestTestResultsGetFailedSample() {
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	rh := s.rh["failed_tests_sample"].(*testResultsGetFailedSampleHandler)

	for _, test := range []struct {
		name           string
		ctx            context.Context
		opts           data.TestResultsOptions
		expectedResult []string
		errorStatus    int
	}{
		{
			name:        "FailsWithContextError",
			ctx:         canceledCtx,
			opts:        data.TestResultsOptions{TaskID: "task1"},
			errorStatus: http.StatusInternalServerError,
		},
		{
			name:        "FailsWhenTaskIDDNE",
			opts:        data.TestResultsOptions{TaskID: "DNE"},
			errorStatus: http.StatusNotFound,
		},
		{
			name:        "FailsWhenDisplayTaskIDDNE",
			opts:        data.TestResultsOptions{TaskID: "DNE", DisplayTask: true},
			errorStatus: http.StatusNotFound,
		},
		{
			name: "SucceedsWithTaskIDAndExecution",
			opts: data.TestResultsOptions{
				TaskID:    "task1",
				Execution: utility.ToIntPtr(0),
			},
			expectedResult: []string{"test0", "test1", "test2"},
		},
		{
			name:           "SucceedsWithTaskIDAndNoExecution",
			opts:           data.TestResultsOptions{TaskID: "task1"},
			expectedResult: []string{"test0", "test1", "test2"},
		},
		{
			name: "SucceedsWithDisplayTaskIDAndExecution",
			opts: data.TestResultsOptions{
				TaskID:      "display_task1",
				Execution:   utility.ToIntPtr(0),
				DisplayTask: true,
			},
			expectedResult: []string{"test0", "test1", "test2", "test0", "test1", "test2"},
		},
		{
			name: "SucceedsWithDisplayTaskIDAndExecution",
			opts: data.TestResultsOptions{
				TaskID:      "display_task1",
				DisplayTask: true,
			},
			expectedResult: []string{"test0", "test1", "test2"},
		},
	} {
		s.T().Run(test.name, func(t *testing.T) {
			rh.opts = test.opts
			ctx := test.ctx
			if ctx == nil {
				ctx = context.TODO()
			}
			resp := rh.Run(ctx)

			s.Require().NotNil(resp)
			if test.errorStatus > 0 {
				s.Equal(test.errorStatus, resp.Status())
			} else {
				s.Equal(http.StatusOK, resp.Status())
				actualResult, ok := resp.Data().([]string)
				s.Require().True(ok)
				s.Equal(test.expectedResult, actualResult)
			}
		})
	}
}

func (s *TestResultsHandlerSuite) TestTestResultsGetStats() {
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	rh := s.rh["stats"].(*testResultsGetStatsHandler)

	for _, test := range []struct {
		name           string
		ctx            context.Context
		opts           data.TestResultsOptions
		expectedResult *model.APITestResultsStats
		errorStatus    int
	}{
		{
			name:        "FailsWithContextError",
			ctx:         canceledCtx,
			opts:        data.TestResultsOptions{TaskID: "task1"},
			errorStatus: http.StatusInternalServerError,
		},
		{
			name:        "FailsWhenTaskIDDNE",
			opts:        data.TestResultsOptions{TaskID: "DNE"},
			errorStatus: http.StatusNotFound,
		},
		{
			name:        "FailsWhenDisplayTaskIDDNE",
			opts:        data.TestResultsOptions{TaskID: "DNE", DisplayTask: true},
			errorStatus: http.StatusNotFound,
		},
		{
			name: "SucceedsWithTaskIDAndExecution",
			opts: data.TestResultsOptions{
				TaskID:    "task1",
				Execution: utility.ToIntPtr(0),
			},
			expectedResult: &model.APITestResultsStats{
				TotalCount:  3,
				FailedCount: 3,
			},
		},
		{
			name: "SucceedsWithTaskIDAndNoExecution",
			opts: data.TestResultsOptions{TaskID: "task1"},
			expectedResult: &model.APITestResultsStats{
				TotalCount:  3,
				FailedCount: 3,
			},
		},
		{
			name: "SucceedsWithDisplayTaskIDAndExecution",
			opts: data.TestResultsOptions{
				TaskID:      "display_task1",
				Execution:   utility.ToIntPtr(0),
				DisplayTask: true,
			},
			expectedResult: &model.APITestResultsStats{
				TotalCount:  6,
				FailedCount: 6,
			},
		},
		{
			name: "SucceedsWithDisplayTaskIDAndExecution",
			opts: data.TestResultsOptions{
				TaskID:      "display_task1",
				DisplayTask: true,
			},
			expectedResult: &model.APITestResultsStats{
				TotalCount:  3,
				FailedCount: 3,
			},
		},
	} {
		s.T().Run(test.name, func(t *testing.T) {
			rh.opts = test.opts
			ctx := test.ctx
			if ctx == nil {
				ctx = context.TODO()
			}
			resp := rh.Run(ctx)

			s.Require().NotNil(resp)
			if test.errorStatus > 0 {
				s.Equal(test.errorStatus, resp.Status())
			} else {
				s.Equal(http.StatusOK, resp.Status())
				actualResult, ok := resp.Data().(*model.APITestResultsStats)
				s.Require().True(ok)
				s.Equal(test.expectedResult, actualResult)
			}
		})
	}
}

func (s *TestResultsHandlerSuite) TestTestResultsGetByDisplayTaskIDHandler() {
	rh := s.rh["display_task_id"].(*testResultsGetByDisplayTaskIDHandler)
	optsList := []data.TestResultsOptions{
		{
			TaskID:      "display_task1",
			Execution:   utility.ToIntPtr(0),
			DisplayTask: true,
		},
		{
			TaskID:      "display_task1",
			DisplayTask: true,
		},
	}
	expectedResults := [][]model.APITestResult{
		append(s.apiResults["abc"], s.apiResults["ghi"]...),
		s.apiResults["def"],
	}

	for i, opts := range optsList {
		rh.opts = opts

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
	rh := s.rh["display_task_id"].(*testResultsGetByDisplayTaskIDHandler)
	rh.opts.TaskID = "DNE"
	rh.opts.DisplayTask = true

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusNotFound, resp.Status())
}

func (s *TestResultsHandlerSuite) TestTestResultsGetByDisplayTaskIDHandlerCtxErr() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rh := s.rh["display_task_id"].(*testResultsGetByDisplayTaskIDHandler)
	rh.opts.TaskID = "display_task1"
	rh.opts.DisplayTask = true

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.Equal(http.StatusInternalServerError, resp.Status())
}

func (s *TestResultsHandlerSuite) TestTestResultGetByTestNameHandlerFound() {
	rh := s.rh["test_name"].(*testResultGetByTestNameHandler)
	rh.opts.TaskID = "task1"
	rh.opts.FilterAndSort = &data.TestResultsFilterAndSortOptions{TestName: "test1"}
	rh.opts.Execution = utility.ToIntPtr(0)

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
	rh := s.rh["test_name"].(*testResultGetByTestNameHandler)
	rh.opts.TaskID = "task1"
	rh.opts.FilterAndSort = &data.TestResultsFilterAndSortOptions{TestName: "DNE"}

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusNotFound, resp.Status())
}

func (s *TestResultsHandlerSuite) TestTestResultGetByTestNameHandlerCtxErr() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rh := s.rh["test_name"].(*testResultGetByTestNameHandler)
	rh.opts.TaskID = "task1"
	rh.opts.FilterAndSort = &data.TestResultsFilterAndSortOptions{TestName: "test1"}

	resp := rh.Run(ctx)
	s.Require().NotNil(resp)
	s.Equal(http.StatusInternalServerError, resp.Status())
}

func (s *TestResultsHandlerSuite) TestBaseParse() {
	for _, test := range []struct {
		urlString string
		handler   string
	}{
		{
			handler:   "task_id",
			urlString: "http://cedar.mongodb.com/test_results/task_id/task_id1",
		},
		{
			handler:   "failed_tests_sample",
			urlString: "http://cedar.mongodb.com/test_results/task_id/task_id1/failed_sample",
		},
		{
			handler:   "stats",
			urlString: "http://cedar.mongodb.com/test_results/task_id/task_id1/stats",
		},
	} {
		s.testParseValid(test.handler, test.urlString)
		s.testParseInvalid(test.handler, test.urlString)
		s.testParseDefaults(test.handler, test.urlString)
	}
}

func (s *TestResultsHandlerSuite) testParseValid(handler, urlString string) {
	ctx := context.Background()
	urlString += "?execution=1&display_task=true"
	req := &http.Request{Method: "GET"}
	req.URL, _ = url.Parse(urlString)
	rh := s.rh[handler]
	// Need to reset this since we are reusing the handlers.
	rh = rh.Factory()

	err := rh.Parse(ctx, req)
	s.Require().NoError(err)
	s.Equal(1, utility.FromIntPtr(getTestResultsExecution(rh, handler)))
	s.True(getTestResultsDisplayTask(rh, handler))
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
	s.Nil(getTestResultsExecution(rh, handler))
}

func getTestResultsExecution(rh gimlet.RouteHandler, handler string) *int {
	switch handler {
	case "task_id":
		return rh.(*testResultsGetByTaskIDHandler).opts.Execution
	case "failed_tests_sample":
		return rh.(*testResultsGetFailedSampleHandler).opts.Execution
	case "stats":
		return rh.(*testResultsGetStatsHandler).opts.Execution
	default:
		return nil
	}
}

func getTestResultsDisplayTask(rh gimlet.RouteHandler, handler string) bool {
	switch handler {
	case "task_id":
		return rh.(*testResultsGetByTaskIDHandler).opts.DisplayTask
	case "failed_tests_sample":
		return rh.(*testResultsGetFailedSampleHandler).opts.DisplayTask
	case "stats":
		return rh.(*testResultsGetStatsHandler).opts.DisplayTask
	default:
		return false
	}
}

func (s *TestResultsHandlerSuite) TestTaskIDParse() {
	rh := s.rh["task_id"].Factory().(*testResultsGetByTaskIDHandler)

	// Test default.
	urlString := "http://cedar.mongodb.com/rest/v1/test_results/task_id/task_id1"
	expected := data.TestResultsOptions{}
	req := &http.Request{Method: "GET"}
	req.URL, _ = url.Parse(urlString)

	err := rh.Parse(context.TODO(), req)
	s.Require().NoError(err)
	s.Equal(expected, rh.opts)

	// Test valid query parameters.
	rh = rh.Factory().(*testResultsGetByTaskIDHandler)
	urlString += "?test_name=test&status=fail&status=silentfail&group_id=group&sort_by=sort&sort_order_dsc=true&limit=5&page=2"
	expected = data.TestResultsOptions{
		FilterAndSort: &data.TestResultsFilterAndSortOptions{
			TestName:     "test",
			Statuses:     []string{"fail", "silentfail"},
			GroupID:      "group",
			SortBy:       "sort",
			SortOrderDSC: true,
			Limit:        5,
			Page:         2,
		},
	}
	req = &http.Request{Method: "GET"}
	req.URL, _ = url.Parse(urlString)

	err = rh.Parse(context.TODO(), req)
	s.Require().NoError(err)
	s.Equal(expected, rh.opts)
}
