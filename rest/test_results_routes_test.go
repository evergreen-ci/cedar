package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
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
				TaskID:    testResults.Info.TaskID,
				Execution: testResults.Info.Execution,
				TestName:  fmt.Sprintf("test%d", i),
				Status:    "teststatus-fail",
				LogInfo: &dbModel.TestLogInfo{
					LogName: "log0",
					LineNum: 100,
				},
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
			FilterOpts *data.TestResultsFilterAndSortOptions `json:"filter"`
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
				FilterOpts *data.TestResultsFilterAndSortOptions `json:"filter"`
			}{},
			hasErr: true,
		},
		{
			name: "OnlyTaskOptions",
			payload: &struct {
				TaskOpts   []data.TestResultsTaskOptions         `json:"tasks"`
				FilterOpts *data.TestResultsFilterAndSortOptions `json:"filter"`
			}{
				TaskOpts: []data.TestResultsTaskOptions{
					{
						TaskID:    "task1",
						Execution: 1,
					},
					{
						TaskID:    "task2",
						Execution: 0,
					},
				},
			},
		},
		{
			name: "TaskAndFilterOptions",
			payload: &struct {
				TaskOpts   []data.TestResultsTaskOptions         `json:"tasks"`
				FilterOpts *data.TestResultsFilterAndSortOptions `json:"filter"`
			}{
				TaskOpts: []data.TestResultsTaskOptions{
					{
						TaskID:    "task1",
						Execution: 1,
					},
					{
						TaskID:    "task2",
						Execution: 0,
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
					BaseTasks: []data.TestResultsTaskOptions{
						{
							TaskID:    "base_task1",
							Execution: 0,
						},
						{
							TaskID:    "base_task2",
							Execution: 1,
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

			rh := &testResultsBaseHandler{sc: s.sc}
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
					Execution: 0,
				},
			},
			errorStatus: http.StatusInternalServerError,
		},
		{
			name: "InvalidFilterOptions",
			taskOpts: []data.TestResultsTaskOptions{
				{
					TaskID:    "task1",
					Execution: 1,
				},
				{
					TaskID:    "task2",
					Execution: 0,
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
					Execution: 0,
				},
			},
			expectedResult: &model.APITestResults{
				Stats:   model.APITestResultsStats{FilteredCount: utility.ToIntPtr(0)},
				Results: []model.APITestResult{},
			},
		},
		{
			name: "NoFilters",
			taskOpts: []data.TestResultsTaskOptions{
				{
					TaskID:    "task1",
					Execution: 1,
				},
				{
					TaskID:    "task2",
					Execution: 0,
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
					Execution: 0,
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
					Execution: 1,
				},
				{
					TaskID:    "task2",
					Execution: 0,
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
				s.Equal(test.expectedResult.Results, actualResult.Results)
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
					Execution: 0,
				},
			},
			errorStatus: http.StatusInternalServerError,
		},
		{
			name: "TaskDNE",
			taskOpts: []data.TestResultsTaskOptions{
				{
					TaskID:    "DNE",
					Execution: 0,
				},
			},
			expectedResult: &model.APITestResultsStats{},
		},
		{
			name: "TasksExist",
			taskOpts: []data.TestResultsTaskOptions{
				{
					TaskID:    "task1",
					Execution: 1,
				},
				{
					TaskID:    "task2",
					Execution: 0,
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
					Execution: 0,
				},
			},
			errorStatus: http.StatusInternalServerError,
		},
		{
			name: "TaskDNE",
			taskOpts: []data.TestResultsTaskOptions{
				{
					TaskID:    "DNE",
					Execution: 0,
				},
			},
		},
		{
			name: "TasksExist",
			taskOpts: []data.TestResultsTaskOptions{
				{
					TaskID:    "task1",
					Execution: 1,
				},
				{
					TaskID:    "task2",
					Execution: 0,
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
