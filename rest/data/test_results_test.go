package data

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/pail"
	"github.com/stretchr/testify/suite"
)

type testResultsConnectorSuite struct {
	ctx         context.Context
	cancel      context.CancelFunc
	sc          Connector
	env         cedar.Environment
	testResults map[string]model.TestResults
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
	s.testResults = map[string]model.TestResults{}

	// setup config
	var err error
	s.tempDir, err = ioutil.TempDir(".", "testResults_connector")
	s.Require().NoError(err)
	conf := model.NewCedarConfig(s.env)
	conf.Bucket = model.BucketConfig{TestResultsBucket: s.tempDir}
	s.Require().NoError(conf.Save())

	testResults := []model.TestResultsInfo{
		{
			Project:     "test",
			Version:     "0",
			Variant:     "linux",
			TaskName:    "task0",
			TaskID:      "task1",
			Execution:   0,
			RequestType: "test",
			Mainline:    true,
			Schema:      0,
		},
		{
			Project:     "test",
			Version:     "0",
			Variant:     "linux",
			TaskName:    "task0",
			TaskID:      "task1",
			Execution:   1,
			RequestType: "test",
			Mainline:    true,
			Schema:      0,
		},
		{
			Project:     "test",
			Version:     "0",
			Variant:     "linux",
			TaskName:    "task2",
			TaskID:      "task3",
			Execution:   0,
			RequestType: "test",
			Mainline:    true,
			Schema:      0,
		},
		{
			Project:     "test",
			Version:     "0",
			Variant:     "linux",
			TaskName:    "task2",
			TaskID:      "task3",
			Execution:   1,
			RequestType: "test",
			Mainline:    true,
			Schema:      0,
		},
		{
			Project:     "test",
			Version:     "0",
			Variant:     "linux",
			TaskName:    "task2",
			TaskID:      "task3",
			Execution:   2,
			RequestType: "test",
			Mainline:    true,
			Schema:      0,
		},
	}

	for _, testResultsInfo := range testResults {
		tr := model.CreateTestResults(testResultsInfo, model.PailLocal)

		opts := pail.LocalOptions{
			Path:   s.tempDir,
			Prefix: tr.Artifact.Prefix,
		}
		bucket, err := pail.NewLocalBucket(opts)
		s.Require().NoError(err)

		// create corresponding test results

		tr.Setup(s.env)
		s.Require().NoError(tr.SaveNew(s.ctx))
		s.testResults[tr.ID] = *tr
		time.Sleep(time.Second)
	}
}

func (s *testResultsConnectorSuite) TearDownSuite() {
	defer s.cancel()
	s.NoError(os.RemoveAll(s.tempDir))
	s.NoError(s.env.GetDB().Drop(s.ctx))
}
