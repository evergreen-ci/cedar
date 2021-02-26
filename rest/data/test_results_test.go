package data

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/pail"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson"
)

type testResultsConnectorSuite struct {
	ctx         context.Context
	cancel      context.CancelFunc
	sc          Connector
	env         cedar.Environment
	testResults map[string]dbModel.TestResults
	tempDir     string
	suite.Suite
	apiResults map[string]model.APITestResult
}

func TestTestResultsConnectorSuiteDB(t *testing.T) {
	s := new(testResultsConnectorSuite)
	s.setup()
	s.sc = CreateNewDBConnector(s.env)
	suite.Run(t, s)
}

func TestTestResultsConnectorSuiteMock(t *testing.T) {
	s := new(testResultsConnectorSuite)
	s.setup()
	s.sc = &MockConnector{
		CachedTestResults: s.testResults,
		env:               cedar.GetEnvironment(),
		Bucket:            s.tempDir,
	}
	suite.Run(t, s)
}

func (s *testResultsConnectorSuite) setup() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.env = cedar.GetEnvironment()
	s.Require().NotNil(s.env)
	db := s.env.GetDB()
	s.Require().NotNil(db)
	s.testResults = map[string]dbModel.TestResults{}

	// setup config
	var err error
	s.tempDir, err = ioutil.TempDir(".", "testResults_connector")
	s.Require().NoError(err)
	conf := dbModel.NewCedarConfig(s.env)
	conf.Bucket = dbModel.BucketConfig{TestResultsBucket: s.tempDir}
	s.Require().NoError(conf.Save())

	testResultInfos := []dbModel.TestResultsInfo{
		{
			Project:     "test",
			Version:     "0",
			Variant:     "linux",
			TaskName:    "task0",
			TaskID:      "task1",
			Execution:   0,
			RequestType: "requesttype",
			Mainline:    true,
		},
		{
			Project:     "test",
			Version:     "0",
			Variant:     "linux",
			TaskName:    "task0",
			TaskID:      "task1",
			Execution:   1,
			RequestType: "requesttype",
			Mainline:    true,
		},
		{
			Project:     "test",
			Version:     "0",
			Variant:     "linux",
			TaskName:    "task2",
			TaskID:      "task3",
			Execution:   0,
			RequestType: "requesttype",
			Mainline:    true,
		},
	}

	s.apiResults = map[string]model.APITestResult{}

	for _, testResultsInfo := range testResultInfos {
		testResults := dbModel.CreateTestResults(testResultsInfo, dbModel.PailLocal)

		opts := pail.LocalOptions{
			Path:   s.tempDir,
			Prefix: testResults.Artifact.Prefix,
		}
		bucket, err := pail.NewLocalBucket(opts)
		s.Require().NoError(err)

		testResults.Setup(s.env)
		err = testResults.SaveNew(s.ctx)

		s.Require().NoError(err)
		s.testResults[testResults.ID] = *testResults

		for i := 0; i < 3; i++ {
			result := dbModel.TestResult{
				TaskID:         testResults.Info.TaskID,
				Execution:      testResults.Info.Execution,
				TestName:       fmt.Sprintf("test%d", i),
				Trial:          0,
				Status:         "teststatus",
				LineNum:        0,
				TaskCreateTime: time.Now().Add(-3 * time.Second),
				TestStartTime:  time.Now().Add(-2 * time.Second),
				TestEndTime:    time.Now().Add(-1 * time.Second),
			}

			apiResult := model.APITestResult{}
			s.Require().NoError(apiResult.Import(result))
			s.apiResults[fmt.Sprintf("%s_%d_%s", result.TaskID, result.Execution, result.TestName)] = apiResult

			data, err := bson.Marshal(result)
			s.Require().NoError(err)
			s.Require().NoError(bucket.Put(s.ctx, result.TestName, bytes.NewReader(data)))
		}
	}
}

func (s *testResultsConnectorSuite) TearDownSuite() {
	defer s.cancel()
	s.NoError(os.RemoveAll(s.tempDir))
	s.NoError(s.env.GetDB().Drop(s.ctx))
}

func (s *testResultsConnectorSuite) TestFindTestResultsByTaskIDExists() {
	optsList := []TestResultsOptions{{
		TaskID:    "task1",
		Execution: 0,
	}, {
		TaskID:         "task1",
		EmptyExecution: true,
	}}

	expectedResultsList := make([][]model.APITestResult, 0)
	expectedResults := make([]model.APITestResult, 0)
	expectedResultsKeys := [][]string{
		{"task1_0_test0", "task1_0_test1", "task1_0_test2"},
		{"task1_1_test0", "task1_1_test1", "task1_1_test2"},
	}

	for _, testNum := range expectedResultsKeys {
		for _, key := range testNum {
			expectedResults = append(expectedResults, s.apiResults[key])
		}
		expectedResultsList = append(expectedResultsList, expectedResults)
		expectedResults = nil
	}

	for i, opts := range optsList {
		expected := expectedResultsList[i]

		actual, err := s.sc.FindTestResultsByTaskID(s.ctx, opts)
		s.Require().NoError(err)

		s.Len(expected, len(actual))
		for j := 0; j < len(actual); j++ {
			s.Equal(expected[j].TestName, actual[j].TestName)
			s.Equal(expected[j].TaskID, actual[j].TaskID)
			s.Equal(expected[j].Execution, actual[j].Execution)
		}
	}
}

func (s *testResultsConnectorSuite) TestFindTestResultByTaskIDDNE() {
	opts := TestResultsOptions{
		TaskID:    "DNE",
		Execution: 1,
	}

	result, err := s.sc.FindTestResultsByTaskID(s.ctx, opts)
	s.Error(err)
	s.Nil(result)
}

func (s *testResultsConnectorSuite) TestFindTestResultByTaskIDEmpty() {
	opts := TestResultsOptions{Execution: 1}

	result, err := s.sc.FindTestResultsByTaskID(s.ctx, opts)
	s.Error(err)
	s.Nil(result)
}

func (s *testResultsConnectorSuite) TestFindTestResultByTestNameExists() {
	optsList := []TestResultsOptions{{
		TaskID:    "task1",
		Execution: 1,
		TestName:  "test1",
	}, {
		TaskID:         "task1",
		EmptyExecution: true,
		TestName:       "test1",
	}}
	for _, opts := range optsList {
		findOpts := dbModel.TestResultsFindOptions{
			TaskID:         opts.TaskID,
			Execution:      opts.Execution,
			EmptyExecution: opts.EmptyExecution,
		}
		results, err := dbModel.FindTestResults(s.ctx, s.env, findOpts)
		s.Require().NoError(err)
		bucket, err := results[0].GetBucket(s.ctx)
		s.Require().NoError(err)

		tr, err := bucket.Get(s.ctx, opts.TestName)
		s.Require().NoError(err)
		defer func() {
			s.NoError(tr.Close())
		}()

		data, err := ioutil.ReadAll(tr)
		s.Require().NoError(err)

		var result dbModel.TestResult
		s.Require().NoError(bson.Unmarshal(data, &result))
		expected := &model.APITestResult{}
		s.Require().NoError(expected.Import(result))

		actual, err := s.sc.FindTestResultByTestName(s.ctx, opts)
		s.Require().NoError(err)
		s.Equal(expected, actual)
	}
}

func (s *testResultsConnectorSuite) TestFindTestResultByTestNameDNE() {
	// Test when metadata object doesn't exist.
	opts := TestResultsOptions{
		TaskID:    "DNE",
		Execution: 1,
		TestName:  "test1",
	}

	result, err := s.sc.FindTestResultByTestName(s.ctx, opts)
	s.Error(err)
	s.Nil(result)

	// Test when test object doesn't exist.
	opts = TestResultsOptions{
		TaskID:    "task1",
		Execution: 1,
		TestName:  "DNE",
	}

	result, err = s.sc.FindTestResultByTestName(s.ctx, opts)
	s.Error(err)
	s.Nil(result)
}

func (s *testResultsConnectorSuite) TestFindTestResultByTestNameEmpty() {
	opts := TestResultsOptions{
		TaskID:    "task1",
		Execution: 1,
	}

	result, err := s.sc.FindTestResultByTestName(s.ctx, opts)
	s.Error(err)
	s.Nil(result)
}
