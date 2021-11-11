package data

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/evergreen-ci/cedar"
	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/utility"
	"github.com/stretchr/testify/suite"
)

type testResultsConnectorSuite struct {
	ctx        context.Context
	cancel     context.CancelFunc
	sc         Connector
	env        cedar.Environment
	tempDir    string
	apiResults map[string]model.APITestResult

	suite.Suite
}

func TestTestResultsConnectorSuiteDB(t *testing.T) {
	s := new(testResultsConnectorSuite)
	suite.Run(t, s)
}

func (s *testResultsConnectorSuite) setupData() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.env = cedar.GetEnvironment()
	s.Require().NotNil(s.env)
	db := s.env.GetDB()
	s.Require().NotNil(db)

	// setup config
	var err error
	s.tempDir, err = ioutil.TempDir(".", "testResults_connector")
	s.Require().NoError(err)
	conf := dbModel.NewCedarConfig(s.env)
	conf.Bucket = dbModel.BucketConfig{TestResultsBucket: s.tempDir}
	s.Require().NoError(conf.Save())

	testResultInfos := []dbModel.TestResultsInfo{
		{
			Project:       "test",
			Version:       "0",
			Variant:       "linux",
			TaskID:        "task1",
			DisplayTaskID: "display_task1",
			Execution:     0,
			RequestType:   "requesttype",
			Mainline:      true,
		},
		{
			Project:       "test",
			Version:       "0",
			Variant:       "linux",
			TaskID:        "task1",
			DisplayTaskID: "display_task1",
			Execution:     1,
			RequestType:   "requesttype",
			Mainline:      true,
		},
		{
			Project:       "test",
			Version:       "0",
			Variant:       "linux",
			TaskID:        "task2",
			DisplayTaskID: "display_task1",
			Execution:     0,
			RequestType:   "requesttype",
			Mainline:      true,
		},
		{
			Project:       "test",
			Version:       "0",
			Variant:       "linux",
			TaskID:        "task3",
			DisplayTaskID: "display_task2",
			Execution:     0,
			RequestType:   "requesttype",
			Mainline:      true,
		},
	}

	s.apiResults = map[string]model.APITestResult{}

	for _, testResultsInfo := range testResultInfos {
		testResults := dbModel.CreateTestResults(testResultsInfo, dbModel.PailLocal)

		testResults.Setup(s.env)
		err = testResults.SaveNew(s.ctx)
		s.Require().NoError(err)

		for i := 0; i < 3; i++ {
			result := dbModel.TestResult{
				TaskID:    testResults.Info.TaskID,
				Execution: testResults.Info.Execution,
				TestName:  fmt.Sprintf("test%d", i),
				Trial:     0,
				Status:    "teststatus-fail",
				LineNum:   0,
			}

			apiResult := model.APITestResult{}
			s.Require().NoError(apiResult.Import(result))
			s.apiResults[fmt.Sprintf("%s_%d_%s", result.TaskID, result.Execution, result.TestName)] = apiResult

			s.Require().NoError(testResults.Append(s.ctx, []dbModel.TestResult{result}))
		}
	}
}

func (s *testResultsConnectorSuite) SetupSuite() {
	s.setupData()
	s.sc = CreateNewDBConnector(s.env, "")
}

func (s *testResultsConnectorSuite) TearDownSuite() {
	defer s.cancel()
	s.NoError(os.RemoveAll(s.tempDir))
	s.NoError(s.env.GetDB().Drop(s.ctx))
}

func (s *testResultsConnectorSuite) TestFindTestResults() {
	for _, test := range []struct {
		name      string
		opts      TestResultsOptions
		stats     model.APITestResultsStats
		resultMap map[string]model.APITestResult
		hasErr    bool
	}{
		{
			name:   "FailsWithEmptyOptions",
			hasErr: true,
		},
		{
			name:   "FailsWhenTaskIDDNE",
			opts:   TestResultsOptions{TaskID: "DNE"},
			hasErr: true,
		},
		{
			name:   "FailsWhenDisplayTaskIDDNE",
			opts:   TestResultsOptions{TaskID: "DNE", DisplayTask: true},
			hasErr: true,
		},
		{
			name: "SucceedsWithTaskIDAndExecution",
			opts: TestResultsOptions{
				TaskID:    "task1",
				Execution: utility.ToIntPtr(0),
			},
			stats: model.APITestResultsStats{
				TotalCount:    3,
				FailedCount:   3,
				FilteredCount: utility.ToIntPtr(3),
			},
			resultMap: map[string]model.APITestResult{
				"task1_0_test0": s.apiResults["task1_0_test0"],
				"task1_0_test1": s.apiResults["task1_0_test1"],
				"task1_0_test2": s.apiResults["task1_0_test2"],
			},
		},
		{
			name: "SucceedsWithTaskIDAndNoExecution",
			opts: TestResultsOptions{TaskID: "task1"},
			stats: model.APITestResultsStats{
				TotalCount:    3,
				FailedCount:   3,
				FilteredCount: utility.ToIntPtr(3),
			},
			resultMap: map[string]model.APITestResult{
				"task1_1_test0": s.apiResults["task1_1_test0"],
				"task1_1_test1": s.apiResults["task1_1_test1"],
				"task1_1_test2": s.apiResults["task1_1_test2"],
			},
		},
		{
			name: "SucceedsWithTaskIDAndFilterAndSort",
			opts: TestResultsOptions{
				TaskID:    "task1",
				Execution: utility.ToIntPtr(0),
				FilterAndSort: &TestResultsFilterAndSortOptions{
					TestName: "test1",
				},
			},
			stats: model.APITestResultsStats{
				TotalCount:    3,
				FailedCount:   3,
				FilteredCount: utility.ToIntPtr(1),
			},
			resultMap: map[string]model.APITestResult{
				"task1_0_test1": s.apiResults["task1_0_test1"],
			},
		},
		{
			name: "SucceedsWithDisplayTaskIDAndExecution",
			opts: TestResultsOptions{
				TaskID:      "display_task1",
				Execution:   utility.ToIntPtr(0),
				DisplayTask: true,
			},
			stats: model.APITestResultsStats{
				TotalCount:    6,
				FailedCount:   6,
				FilteredCount: utility.ToIntPtr(6),
			},
			resultMap: map[string]model.APITestResult{
				"task1_0_test0": s.apiResults["task1_0_test0"],
				"task1_0_test1": s.apiResults["task1_0_test1"],
				"task1_0_test2": s.apiResults["task1_0_test2"],
				"task2_0_test0": s.apiResults["task2_0_test0"],
				"task2_0_test1": s.apiResults["task2_0_test1"],
				"task2_0_test2": s.apiResults["task2_0_test2"],
			},
		},
		{
			name: "SucceedsWithDisplayTaskIDAndNoExecution",
			opts: TestResultsOptions{
				TaskID:      "display_task1",
				DisplayTask: true,
			},
			stats: model.APITestResultsStats{
				TotalCount:    3,
				FailedCount:   3,
				FilteredCount: utility.ToIntPtr(3),
			},
			resultMap: map[string]model.APITestResult{
				"task1_1_test0": s.apiResults["task1_1_test0"],
				"task1_1_test1": s.apiResults["task1_1_test1"],
				"task1_1_test2": s.apiResults["task1_1_test2"],
			},
		},
		{
			name: "SucceedsWithDisplayTaskIDAndFilterAndSort",
			opts: TestResultsOptions{
				TaskID:    "display_task1",
				Execution: utility.ToIntPtr(0),
				FilterAndSort: &TestResultsFilterAndSortOptions{
					TestName: "test2",
				},
				DisplayTask: true,
			},
			stats: model.APITestResultsStats{
				TotalCount:    6,
				FailedCount:   6,
				FilteredCount: utility.ToIntPtr(2),
			},
			resultMap: map[string]model.APITestResult{
				"task1_0_test2": s.apiResults["task1_0_test2"],
				"task2_0_test2": s.apiResults["task2_0_test2"],
			},
		},
	} {
		s.T().Run(test.name, func(t *testing.T) {
			actualResult, err := s.sc.FindTestResults(s.ctx, test.opts)

			if test.hasErr {
				s.Nil(actualResult)
				s.Error(err)
			} else {
				s.Require().NoError(err)

				s.Equal(test.stats, actualResult.Stats)
				s.Len(actualResult.Results, len(test.resultMap))
				for _, result := range actualResult.Results {
					key := fmt.Sprintf("%s_%d_%s", *result.TaskID, result.Execution, *result.TestName)
					expected, ok := test.resultMap[key]
					s.Require().True(ok)
					s.Equal(expected.TestName, result.TestName)
					s.Equal(expected.TaskID, result.TaskID)
					s.Equal(expected.Execution, result.Execution)
					delete(test.resultMap, key)
				}
			}
		})
	}
}

func (s *testResultsConnectorSuite) TestGetFailedTestResultsSample() {
	for _, test := range []struct {
		name           string
		opts           TestResultsOptions
		expectedResult []string
		hasErr         bool
	}{
		{
			name:   "FailsWithEmptyOptions",
			hasErr: true,
		},
		{
			name:   "FailsWhenTaskIDDNE",
			opts:   TestResultsOptions{TaskID: "DNE"},
			hasErr: true,
		},
		{
			name:   "FailsWhenDisplayTaskIDDNE",
			opts:   TestResultsOptions{TaskID: "DNE", DisplayTask: true},
			hasErr: true,
		},
		{
			name: "TaskIDWithExecution",
			opts: TestResultsOptions{
				TaskID:    "task1",
				Execution: utility.ToIntPtr(0),
			},
			expectedResult: []string{"test0", "test1", "test2"},
		},
		{
			name:           "TaskIDWithoutExecution",
			opts:           TestResultsOptions{TaskID: "task1"},
			expectedResult: []string{"test0", "test1", "test2"},
		},
		{
			name: "DisplayTaskIDWithExecution",
			opts: TestResultsOptions{
				TaskID:      "display_task1",
				Execution:   utility.ToIntPtr(0),
				DisplayTask: true,
			},
			expectedResult: []string{
				"test0",
				"test1",
				"test2",
				"test0",
				"test1",
				"test2",
			},
		},
		{
			name: "DisplayTaskIDWithoutExecution",
			opts: TestResultsOptions{
				TaskID:      "display_task1",
				DisplayTask: true,
			},
			expectedResult: []string{"test0", "test1", "test2"},
		},
	} {
		s.T().Run(test.name, func(t *testing.T) {
			actualResult, err := s.sc.GetFailedTestResultsSample(s.ctx, test.opts)

			if test.hasErr {
				s.Nil(actualResult)
				s.Error(err)
			} else {
				s.Require().NoError(err)

				s.Require().Len(actualResult, len(test.expectedResult))
				for i := range actualResult {
					s.Equal(test.expectedResult[i], actualResult[i])
				}
			}
		})
	}
}

func (s *testResultsConnectorSuite) TestGetTestResultsStats() {
	for _, test := range []struct {
		name           string
		opts           TestResultsOptions
		expectedResult *model.APITestResultsStats
		hasErr         bool
	}{
		{
			name:   "FailsWithEmptyOptions",
			hasErr: true,
		},
		{
			name:   "FailsWhenTaskIDDNE",
			opts:   TestResultsOptions{TaskID: "DNE"},
			hasErr: true,
		},
		{
			name:   "FailsWhenDisplayTaskIDDNE",
			opts:   TestResultsOptions{TaskID: "DNE", DisplayTask: true},
			hasErr: true,
		},
		{
			name: "TaskIDWithExecution",
			opts: TestResultsOptions{
				TaskID:    "task1",
				Execution: utility.ToIntPtr(0),
			},
			expectedResult: &model.APITestResultsStats{
				TotalCount:  3,
				FailedCount: 3,
			},
		},
		{
			name: "TaskIDWithoutExecution",
			opts: TestResultsOptions{TaskID: "task1"},
			expectedResult: &model.APITestResultsStats{
				TotalCount:  3,
				FailedCount: 3,
			},
		},
		{
			name: "DisplayTaskIDWithExecution",
			opts: TestResultsOptions{
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
			name: "DisplayTaskIDWithoutExecution",
			opts: TestResultsOptions{
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
			actualResult, err := s.sc.GetTestResultsStats(s.ctx, test.opts)

			if test.hasErr {
				s.Error(err)
			} else {
				s.Require().NoError(err)
			}
			s.Equal(test.expectedResult, actualResult)
		})
	}
}
