package data

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/pail"
	"github.com/stretchr/testify/suite"
	"gopkg.in/mgo.v2/bson"
)

type testResultsConnectorSuite struct {
	ctx         context.Context
	cancel      context.CancelFunc
	sc          Connector
	env         cedar.Environment
	testResults map[string]dbModel.TestResults
	tempDir     string
	suite.Suite
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

	for _, testResultsInfo := range testResultInfos {
		testResults := dbModel.CreateTestResults(testResultsInfo, dbModel.PailLocal)

		opts := pail.LocalOptions{
			Path:   s.tempDir,
			Prefix: filepath.Join("test_results", testResults.Artifact.Prefix),
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
				LogURL:         "testurl",
				LineNum:        0,
				TaskCreateTime: time.Now().Add(-3 * time.Second),
				TestStartTime:  time.Now().Add(-2 * time.Second),
				TestEndTime:    time.Now().Add(-1 * time.Second),
			}
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

// func (s *testResultsConnectorSuite) TestFindTestResultByTaskId() {
// 	expectedID := s.testResults.TestResultsInfo.TaskID

// 	options := dbModel.TestResultsFindOptions{
// 		TaskID:         h.options.TaskID,
// 		Execution:      h.options.Execution,
// 		EmptyExecution: h.options.EmptyExecution,
// 	}

// 	actualResult, err := s.sc.FindTestResultsByTaskId(s.ctx, options)
// 	s.Require().NoError(err)
// 	s.Equal(expectedID, actualResult.TaskID)

// 	actualResult, err = s.sc.FindTestResultsByTaskId(s.ctx, options)
// 	s.Require().NoError(err)
// 	s.Equal(expectedID, *actualResult.Name)
// }
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
		testResults := dbModel.TestResults{}
		testResults.Setup(s.env)
		findOpts := dbModel.TestResultsFindOptions{
			TaskID:         opts.TaskID,
			Execution:      opts.Execution,
			EmptyExecution: opts.EmptyExecution,
		}
		s.Require().NoError(testResults.FindByTaskID(s.ctx, findOpts))
		bucket, err := testResults.GetBucket(s.ctx)
		s.Require().NoError(err)
		tr, err := bucket.Get(s.ctx, opts.TestName)
		s.Require().NoError(err)
		data, err := ioutil.ReadAll(tr)
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
	// test when metadata object doesn't exist
	opts := TestResultsOptions{
		TaskID:    "DNE",
		Execution: 1,
		TestName:  "test1",
	}

	result, err := s.sc.FindTestResultByTestName(s.ctx, opts)
	s.Error(err)
	s.Nil(result)

	// test when test object doesn't exist
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
