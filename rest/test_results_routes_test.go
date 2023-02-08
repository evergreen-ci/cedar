package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
		DisableCache:       true,
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
	tempDir    string

	suite.Suite
}

func (s *TestResultsHandlerSuite) setup(tempDir string) {
	var err error
	s.env, err = newTestEnv()
	s.Require().NoError(err)

	s.Require().NoError(err)
	conf := dbModel.NewCedarConfig(s.env)
	conf.Bucket = dbModel.BucketConfig{
		TestResultsBucket:       tempDir,
		PrestoBucket:            tempDir,
		PrestoTestResultsPrefix: "presto-test-results",
	}
	s.Require().NoError(conf.Save())

	s.sc = data.CreateNewDBConnector(s.env, "url")
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
				TaskID:         testResults.Info.TaskID,
				Execution:      testResults.Info.Execution,
				TestName:       fmt.Sprintf("test%d", i),
				Trial:          0,
				Status:         "teststatus-fail",
				LineNum:        0,
				TaskCreateTime: time.Now().Truncate(time.Millisecond).UTC(),
				TestStartTime:  time.Now().Truncate(time.Millisecond).UTC(),
				TestEndTime:    time.Now().Truncate(time.Millisecond).UTC(),
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
	}
}

func TestTestResultsHandlerSuite(t *testing.T) {
	s := new(TestResultsHandlerSuite)
	suite.Run(t, s)
}

func (s *TestResultsHandlerSuite) SetupSuite() {
	tempDir, err := ioutil.TempDir(".", "bucket_test")
	s.Require().NoError(err)
	s.tempDir = tempDir
	s.setup(tempDir)
}

func (s *TestResultsHandlerSuite) TearDownSuite() {
	s.NoError(os.RemoveAll(s.tempDir))
	err := tearDownEnv(s.env)
	s.Require().NoError(err)
}

func (s *TestResultsHandlerSuite) TestTestResultsBaseHandler() {
	for _, test := range []struct {
		name    string
		payload *struct {
			TaskOpts   []data.TestResultsTaskOptions         `json:"tasks"`
			FilterOpts *data.TestResultsFilterAndSortOptions `json:"filters"`
		}
		hasErr bool
	}{
		{
			name:   "NoPayload",
			hasErr: true,
		},
		{
			name: "NoTaskOptions",
			payload: &struct {
				TaskOpts   []data.TestResultsTaskOptions         `json:"tasks"`
				FilterOpts *data.TestResultsFilterAndSortOptions `json:"filters"`
			}{},
			hasErr: true,
		},
		{
			name: "OnlyTaskOptions",
			payload: &struct {
				TaskOpts   []data.TestResultsTaskOptions         `json:"tasks"`
				FilterOpts *data.TestResultsFilterAndSortOptions `json:"filters"`
			}{
				TaskOpts: []data.TestResultsTaskOptions{
					{
						TaskID:    "task1",
						Execution: utility.ToIntPtr(1),
					},
					{
						TaskID:    "task2",
						Execution: utility.ToIntPtr(0),
					},
				},
			},
		},
		{
			name: "TaskAndFilterOptions",
			payload: &struct {
				TaskOpts   []data.TestResultsTaskOptions         `json:"tasks"`
				FilterOpts *data.TestResultsFilterAndSortOptions `json:"filters"`
			}{
				TaskOpts: []data.TestResultsTaskOptions{
					{
						TaskID:    "task1",
						Execution: utility.ToIntPtr(1),
					},
					{
						TaskID:    "task2",
						Execution: utility.ToIntPtr(0),
					},
				},
				FilterOpts: &data.TestResultsFilterAndSortOptions{
					TestName:     "test1",
					Statuses:     []string{"fail"},
					GroupID:      "group",
					SortBy:       "start",
					SortOrderDSC: true,
					Limit:        100,
					Page:         1,
					BaseResults: []data.TestResultsTaskOptions{
						{
							TaskID:    "base_task1",
							Execution: utility.ToIntPtr(0),
						},
						{
							TaskID:    "base_task2",
							Execution: utility.ToIntPtr(1),
						},
					},
				},
			},
		},
	} {
		s.Run(test.name, func() {
			var body io.ReadCloser
			if test.payload != nil {
				data, err := json.Marshal(test.payload)
				s.Require().NoError(err)
				body = io.NopCloser(bytes.NewReader(data))
			}
			req := &http.Request{Body: body}

			rh := &testResultsTasksBaseHandler{sc: s.sc}
			err := rh.Parse(context.Background(), req)
			if test.hasErr {
				s.Error(err)
			} else {
				s.Require().NoError(err)
				s.Equal(*test.payload, rh.payload)
			}
		})
	}
}

func (s *TestResultsHandlerSuite) TestTestResultsGetByTasksHandler() {
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	for _, test := range []struct {
		name           string
		ctx            context.Context
		taskOpts       []data.TestResultsTaskOptions
		filterOpts     *data.TestResultsFilterAndSortOptions
		expectedResult *model.APITestResults
		errorStatus    int
	}{
		{
			name: "ContextError",
			ctx:  canceledCtx,
			taskOpts: []data.TestResultsTaskOptions{
				{
					TaskID:    "task1",
					Execution: utility.ToIntPtr(0),
				},
			},
			errorStatus: http.StatusInternalServerError,
		},
		{
			name: "InvalidFilterOptions",
			taskOpts: []data.TestResultsTaskOptions{
				{
					TaskID:    "task1",
					Execution: utility.ToIntPtr(1),
				},
				{
					TaskID:    "task2",
					Execution: utility.ToIntPtr(0),
				},
			},
			filterOpts:  &data.TestResultsFilterAndSortOptions{SortBy: "invalid_sort"},
			errorStatus: http.StatusBadRequest,
		},
		{
			name: "TaskDNE",
			taskOpts: []data.TestResultsTaskOptions{
				{
					TaskID:    "DNE",
					Execution: utility.ToIntPtr(0),
				},
			},
			expectedResult: &model.APITestResults{
				Stats: model.APITestResultsStats{FilteredCount: utility.ToIntPtr(0)},
			},
		},
		{
			name: "NoFilters",
			taskOpts: []data.TestResultsTaskOptions{
				{
					TaskID:    "task1",
					Execution: utility.ToIntPtr(1),
				},
				{
					TaskID:    "task2",
					Execution: utility.ToIntPtr(0),
				},
			},
			expectedResult: &model.APITestResults{
				Stats: model.APITestResultsStats{
					TotalCount:    6,
					FailedCount:   6,
					FilteredCount: utility.ToIntPtr(6),
				},
				Results: append(append([]model.APITestResult{}, s.apiResults["def"]...), s.apiResults["ghi"]...),
			},
		},
		{
			name: "Filters",
			taskOpts: []data.TestResultsTaskOptions{
				{
					TaskID:    "task1",
					Execution: utility.ToIntPtr(0),
				},
			},
			filterOpts: &data.TestResultsFilterAndSortOptions{TestName: "test1"},
			expectedResult: &model.APITestResults{
				Stats: model.APITestResultsStats{
					TotalCount:    3,
					FailedCount:   3,
					FilteredCount: utility.ToIntPtr(1),
				},
				Results: s.apiResults["abc"][1:2],
			},
		},
		{
			name: "Paginated",
			taskOpts: []data.TestResultsTaskOptions{
				{
					TaskID:    "task1",
					Execution: utility.ToIntPtr(1),
				},
				{
					TaskID:    "task2",
					Execution: utility.ToIntPtr(0),
				},
			},
			filterOpts: &data.TestResultsFilterAndSortOptions{Limit: 4},
			expectedResult: &model.APITestResults{
				Stats: model.APITestResultsStats{
					TotalCount:    6,
					FailedCount:   6,
					FilteredCount: utility.ToIntPtr(6),
				},
				Results: append(append([]model.APITestResult{}, s.apiResults["def"]...), s.apiResults["ghi"][0:1]...),
			},
		},
	} {
		s.Run(test.name, func() {
			ctx := test.ctx
			if ctx == nil {
				ctx = context.Background()
			}
			rh := makeGetTestResultsByTasks(s.sc)
			rh.payload.TaskOpts = test.taskOpts
			rh.payload.FilterOpts = test.filterOpts
			resp := rh.Run(ctx)

			s.Require().NotNil(resp)
			if test.errorStatus > 0 {
				s.Equal(test.errorStatus, resp.Status())
			} else {
				s.Require().Equal(http.StatusOK, resp.Status())
				actualResult, ok := resp.Data().(*model.APITestResults)
				s.Require().True(ok)
				s.Equal(test.expectedResult.Stats.FailedCount, actualResult.Stats.FailedCount)
				s.Equal(test.expectedResult.Stats.TotalCount, actualResult.Stats.TotalCount)
				s.Equal(test.expectedResult.Stats.FilteredCount, actualResult.Stats.FilteredCount)
				s.Require().Equal(len(test.expectedResult.Results), len(actualResult.Results))
				for _, expected := range test.expectedResult.Results {
					found := false
					for _, actual := range actualResult.Results {
						if utility.FromStringPtr(expected.TestName) == utility.FromStringPtr(actual.TestName) &&
							utility.FromStringPtr(expected.TaskID) == utility.FromStringPtr(actual.TaskID) &&
							expected.Execution == actual.Execution {
							found = true
							break
						}
					}
					s.True(found)
				}
			}
		})
	}
}

func (s *TestResultsHandlerSuite) TestTestResultsGetStatsByTasksHandler() {
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	for _, test := range []struct {
		name           string
		ctx            context.Context
		taskOpts       []data.TestResultsTaskOptions
		expectedResult *model.APITestResultsStats
		errorStatus    int
	}{
		{
			name: "ContextError",
			ctx:  canceledCtx,
			taskOpts: []data.TestResultsTaskOptions{
				{
					TaskID:    "task1",
					Execution: utility.ToIntPtr(0),
				},
			},
			errorStatus: http.StatusInternalServerError,
		},
		{
			name: "TaskDNE",
			taskOpts: []data.TestResultsTaskOptions{
				{
					TaskID:    "DNE",
					Execution: utility.ToIntPtr(0),
				},
			},
			expectedResult: &model.APITestResultsStats{},
		},
		{
			name: "TasksExist",
			taskOpts: []data.TestResultsTaskOptions{
				{
					TaskID:    "task1",
					Execution: utility.ToIntPtr(1),
				},
				{
					TaskID:    "task2",
					Execution: utility.ToIntPtr(0),
				},
			},
			expectedResult: &model.APITestResultsStats{
				TotalCount:  6,
				FailedCount: 6,
			},
		},
	} {
		s.Run(test.name, func() {
			ctx := test.ctx
			if ctx == nil {
				ctx = context.Background()
			}
			rh := makeGetTestResultsStatsByTasks(s.sc)
			rh.payload.TaskOpts = test.taskOpts
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

func (s *TestResultsHandlerSuite) TestTestResultsGetFailedSampleByTasksHandler() {
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	for _, test := range []struct {
		name           string
		ctx            context.Context
		taskOpts       []data.TestResultsTaskOptions
		expectedResult []string
		errorStatus    int
	}{
		{
			name: "ContextError",
			ctx:  canceledCtx,
			taskOpts: []data.TestResultsTaskOptions{
				{
					TaskID:    "task1",
					Execution: utility.ToIntPtr(0),
				},
			},
			errorStatus: http.StatusInternalServerError,
		},
		{
			name: "TaskDNE",
			taskOpts: []data.TestResultsTaskOptions{
				{
					TaskID:    "DNE",
					Execution: utility.ToIntPtr(0),
				},
			},
		},
		{
			name: "TasksExist",
			taskOpts: []data.TestResultsTaskOptions{
				{
					TaskID:    "task1",
					Execution: utility.ToIntPtr(1),
				},
				{
					TaskID:    "task2",
					Execution: utility.ToIntPtr(0),
				},
			},
			expectedResult: []string{"test0", "test1", "test2", "test0", "test1", "test2"},
		},
	} {
		s.Run(test.name, func() {
			ctx := test.ctx
			if ctx == nil {
				ctx = context.TODO()
			}
			rh := makeGetTestResultsFailedSampleByTasks(s.sc)
			rh.payload.TaskOpts = test.taskOpts
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

func (s *TestResultsHandlerSuite) TestTestResultsGetByTaskIDHandler() {
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	rh := s.rh["task_id"].(*testResultsGetByTaskIDHandler)

	for _, test := range []struct {
		name           string
		ctx            context.Context
		taskOpts       data.TestResultsTaskOptions
		filterOpts     *data.TestResultsFilterAndSortOptions
		expectedResult *model.APITestResults
		errorStatus    int
	}{
		{
			name:        "ContextError",
			ctx:         canceledCtx,
			taskOpts:    data.TestResultsTaskOptions{TaskID: "task1"},
			errorStatus: http.StatusInternalServerError,
		},
		{
			name:     "TaskIDDNE",
			taskOpts: data.TestResultsTaskOptions{TaskID: "DNE"},
			expectedResult: &model.APITestResults{
				Stats: model.APITestResultsStats{FilteredCount: utility.ToIntPtr(0)},
			},
		},
		{
			name: "TaskIDAndExecution",
			taskOpts: data.TestResultsTaskOptions{
				TaskID:    "task1",
				Execution: utility.ToIntPtr(0),
			},
			expectedResult: &model.APITestResults{
				Stats: model.APITestResultsStats{
					TotalCount:    3,
					FailedCount:   3,
					FilteredCount: utility.ToIntPtr(3),
				},
				Results: s.apiResults["abc"],
			},
		},
		{
			name:     "TaskIDWithoutExecution",
			taskOpts: data.TestResultsTaskOptions{TaskID: "task1"},
			expectedResult: &model.APITestResults{
				Stats: model.APITestResultsStats{
					TotalCount:    3,
					FailedCount:   3,
					FilteredCount: utility.ToIntPtr(3),
				},
				Results: s.apiResults["def"],
			},
		},
		{
			name: "TaskIDAndFilterAndSort",
			taskOpts: data.TestResultsTaskOptions{
				TaskID:    "task1",
				Execution: utility.ToIntPtr(0),
			},
			filterOpts: &data.TestResultsFilterAndSortOptions{TestName: "test1"},
			expectedResult: &model.APITestResults{
				Stats: model.APITestResultsStats{
					TotalCount:    3,
					FailedCount:   3,
					FilteredCount: utility.ToIntPtr(1),
				},
				Results: s.apiResults["abc"][1:2],
			},
		},
		{
			name: "DisplayTaskIDAndExecution",
			taskOpts: data.TestResultsTaskOptions{
				TaskID:      "display_task1",
				Execution:   utility.ToIntPtr(1),
				DisplayTask: true,
			},
			expectedResult: &model.APITestResults{
				Stats: model.APITestResultsStats{
					TotalCount:    6,
					FailedCount:   6,
					FilteredCount: utility.ToIntPtr(6),
				},
				Results: append(append([]model.APITestResult{}, s.apiResults["def"]...), s.apiResults["ghi"]...),
			},
		},
		{
			name: "DisplayTaskIDAndNoExecution",
			taskOpts: data.TestResultsTaskOptions{
				TaskID:      "display_task1",
				DisplayTask: true,
			},
			expectedResult: &model.APITestResults{
				Stats: model.APITestResultsStats{
					TotalCount:    6,
					FailedCount:   6,
					FilteredCount: utility.ToIntPtr(6),
				},
				Results: append(append([]model.APITestResult{}, s.apiResults["def"]...), s.apiResults["ghi"]...),
			},
		},
		{
			name: "DisplayTaskIDAndFilterAndSort",
			taskOpts: data.TestResultsTaskOptions{
				TaskID:      "display_task1",
				DisplayTask: true,
			},
			filterOpts: &data.TestResultsFilterAndSortOptions{TestName: "test1"},
			expectedResult: &model.APITestResults{
				Stats: model.APITestResultsStats{
					TotalCount:    6,
					FailedCount:   6,
					FilteredCount: utility.ToIntPtr(2),
				},
				Results: []model.APITestResult{s.apiResults["def"][1], s.apiResults["ghi"][1]},
			},
		},
	} {
		s.Run(test.name, func() {
			rh.taskOpts = test.taskOpts
			rh.filterOpts = test.filterOpts
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
				actualResult, ok := resp.Data().(*model.APITestResults)
				s.Require().True(ok)
				s.Equal(test.expectedResult.Stats.FailedCount, actualResult.Stats.FailedCount)
				s.Equal(test.expectedResult.Stats.TotalCount, actualResult.Stats.TotalCount)
				s.Equal(test.expectedResult.Stats.FilteredCount, actualResult.Stats.FilteredCount)
				s.Equal(len(test.expectedResult.Results), len(actualResult.Results))
				for _, expected := range test.expectedResult.Results {
					found := false
					for _, actual := range actualResult.Results {
						if utility.FromStringPtr(expected.TestName) == utility.FromStringPtr(actual.TestName) &&
							utility.FromStringPtr(expected.TaskID) == utility.FromStringPtr(actual.TaskID) &&
							expected.Execution == actual.Execution {
							found = true
							break
						}
					}
					s.True(found)
				}
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
		opts           data.TestResultsTaskOptions
		expectedResult []string
		errorStatus    int
	}{
		{
			name:        "ContextError",
			ctx:         canceledCtx,
			opts:        data.TestResultsTaskOptions{TaskID: "task1"},
			errorStatus: http.StatusInternalServerError,
		},
		{
			name: "TaskIDDNE",
			opts: data.TestResultsTaskOptions{TaskID: "DNE"},
		},
		{
			name: "TaskIDAndExecution",
			opts: data.TestResultsTaskOptions{
				TaskID:    "task1",
				Execution: utility.ToIntPtr(0),
			},
			expectedResult: []string{"test0", "test1", "test2"},
		},
		{
			name:           "TaskIDAndNoExecution",
			opts:           data.TestResultsTaskOptions{TaskID: "task1"},
			expectedResult: []string{"test0", "test1", "test2"},
		},
		{
			name: "DisplayTaskIDAndExecution",
			opts: data.TestResultsTaskOptions{
				TaskID:      "display_task1",
				Execution:   utility.ToIntPtr(0),
				DisplayTask: true,
			},
			expectedResult: []string{"test0", "test1", "test2", "test0", "test1", "test2"},
		},
		{
			name: "DisplayTaskIDAndExecution",
			opts: data.TestResultsTaskOptions{
				TaskID:      "display_task1",
				DisplayTask: true,
			},
			expectedResult: []string{"test0", "test1", "test2", "test0", "test1", "test2"},
		},
	} {
		s.Run(test.name, func() {
			rh.taskOpts = test.opts
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
		opts           data.TestResultsTaskOptions
		expectedResult *model.APITestResultsStats
		errorStatus    int
	}{
		{
			name:        "ContextError",
			ctx:         canceledCtx,
			opts:        data.TestResultsTaskOptions{TaskID: "task1"},
			errorStatus: http.StatusInternalServerError,
		},
		{
			name:           "TaskIDDNE",
			opts:           data.TestResultsTaskOptions{TaskID: "DNE"},
			expectedResult: &model.APITestResultsStats{},
		},
		{
			name: "TaskIDAndExecution",
			opts: data.TestResultsTaskOptions{
				TaskID:    "task1",
				Execution: utility.ToIntPtr(0),
			},
			expectedResult: &model.APITestResultsStats{
				TotalCount:  3,
				FailedCount: 3,
			},
		},
		{
			name: "TaskIDAndNoExecution",
			opts: data.TestResultsTaskOptions{TaskID: "task1"},
			expectedResult: &model.APITestResultsStats{
				TotalCount:  3,
				FailedCount: 3,
			},
		},
		{
			name: "DisplayTaskIDAndExecution",
			opts: data.TestResultsTaskOptions{
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
			name: "DisplayTaskIDAndExecution",
			opts: data.TestResultsTaskOptions{
				TaskID:      "display_task1",
				DisplayTask: true,
			},
			expectedResult: &model.APITestResultsStats{
				TotalCount:  6,
				FailedCount: 6,
			},
		},
	} {
		s.Run(test.name, func() {
			rh.taskOpts = test.opts
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
	req := &http.Request{Method: http.MethodGet}
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
	req := &http.Request{Method: http.MethodGet}
	rh := s.rh[handler]
	// Need to reset this since we are reusing the handlers.
	rh = rh.Factory()

	req.URL, _ = url.Parse(urlString + invalidExecution)
	err := rh.Parse(ctx, req)
	s.Error(err)
}

func (s *TestResultsHandlerSuite) testParseDefaults(handler, urlString string) {
	ctx := context.Background()
	req := &http.Request{Method: http.MethodGet}
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
		return rh.(*testResultsGetByTaskIDHandler).taskOpts.Execution
	case "failed_tests_sample":
		return rh.(*testResultsGetFailedSampleHandler).taskOpts.Execution
	case "stats":
		return rh.(*testResultsGetStatsHandler).taskOpts.Execution
	default:
		return nil
	}
}

func getTestResultsDisplayTask(rh gimlet.RouteHandler, handler string) bool {
	switch handler {
	case "task_id":
		return rh.(*testResultsGetByTaskIDHandler).taskOpts.DisplayTask
	case "failed_tests_sample":
		return rh.(*testResultsGetFailedSampleHandler).taskOpts.DisplayTask
	case "stats":
		return rh.(*testResultsGetStatsHandler).taskOpts.DisplayTask
	default:
		return false
	}
}

func (s *TestResultsHandlerSuite) TestTaskIDParse() {
	rh := s.rh["task_id"].Factory().(*testResultsGetByTaskIDHandler)

	// Test default.
	urlString := "http://cedar.mongodb.com/rest/v1/test_results/task_id/task_id1"
	expectedTaskOpts := data.TestResultsTaskOptions{}
	req := &http.Request{Method: http.MethodGet}
	req.URL, _ = url.Parse(urlString)

	err := rh.Parse(context.TODO(), req)
	s.Require().NoError(err)
	s.Equal(expectedTaskOpts, rh.taskOpts)
	s.Nil(rh.filterOpts)

	// Test valid query parameters.
	rh = rh.Factory().(*testResultsGetByTaskIDHandler)
	urlString += "?test_name=test&status=fail&status=silentfail&group_id=group&sort_by=sort&sort_order_dsc=true&limit=5&page=2"
	expectedFilterOpts := &data.TestResultsFilterAndSortOptions{
		TestName:     "test",
		Statuses:     []string{"fail", "silentfail"},
		GroupID:      "group",
		SortBy:       "sort",
		SortOrderDSC: true,
		Limit:        5,
		Page:         2,
	}
	req = &http.Request{Method: http.MethodGet}
	req.URL, _ = url.Parse(urlString)

	err = rh.Parse(context.TODO(), req)
	s.Require().NoError(err)
	s.Equal(expectedTaskOpts, rh.taskOpts)
	s.Equal(expectedFilterOpts, rh.filterOpts)
}
