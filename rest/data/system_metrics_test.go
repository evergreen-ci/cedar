package data

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/evergreen-ci/cedar"
	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/stretchr/testify/suite"
)

type systemMetricsConnectorSuite struct {
	ctx           context.Context
	cancel        context.CancelFunc
	sc            Connector
	env           cedar.Environment
	systemMetrics map[string]dbModel.SystemMetrics
	tempDir       string
	suite.Suite
}

func TestSystemMetricsConnectorSuiteDB(t *testing.T) {
	s := new(systemMetricsConnectorSuite)
	s.setup()
	s.sc = CreateNewDBConnector(s.env)
	suite.Run(t, s)
}

func TestSystemMetricsConnectorSuiteMock(t *testing.T) {
	s := new(systemMetricsConnectorSuite)
	s.setup()
	s.sc = &MockConnector{
		CachedSystemMetrics: s.systemMetrics,
		env:                 cedar.GetEnvironment(),
		Bucket:              s.tempDir,
	}
	suite.Run(t, s)
}

func (s *systemMetricsConnectorSuite) setup() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.env = cedar.GetEnvironment()
	s.Require().NotNil(s.env)
	db := s.env.GetDB()
	s.Require().NotNil(db)
	s.systemMetrics = map[string]dbModel.SystemMetrics{}

	// setup config
	var err error
	s.tempDir, err = ioutil.TempDir(".", "system-metrics-connector")
	s.Require().NoError(err)
	conf := dbModel.NewCedarConfig(s.env)
	conf.Bucket = dbModel.BucketConfig{SystemMetricsBucket: s.tempDir}
	s.Require().NoError(conf.Save())

	tasks := []dbModel.SystemMetricsInfo{
		{
			Project:   "test",
			Version:   "0",
			Variant:   "linux",
			TaskName:  "task0",
			TaskID:    "task1",
			Execution: 0,
			Mainline:  true,
		},
		{
			Project:   "test",
			Version:   "0",
			Variant:   "linux",
			TaskName:  "task0",
			TaskID:    "task1",
			Execution: 1,
			Mainline:  true,
		},
		{
			Project:   "test",
			Version:   "0",
			Variant:   "linux",
			TaskName:  "task2",
			TaskID:    "task3",
			Execution: 0,
			Mainline:  true,
		},
	}

	for _, taskInfo := range tasks {
		opts := dbModel.SystemMetricsArtifactOptions{Type: dbModel.PailLocal}
		systemMetrics := dbModel.CreateSystemMetrics(taskInfo, opts)
		systemMetrics.Setup(s.env)
		s.Require().NoError(systemMetrics.SaveNew(s.ctx))
		s.Require().NoError(systemMetrics.Append(s.ctx, "uptime", dbModel.FileText, []byte(fmt.Sprintf("execution %d", taskInfo.Execution))))
		s.Require().NoError(systemMetrics.Find(s.ctx))
		s.systemMetrics[systemMetrics.ID] = *systemMetrics
	}
}

func (s *systemMetricsConnectorSuite) TearDownSuite() {
	defer s.cancel()
	s.NoError(os.RemoveAll(s.tempDir))
	s.NoError(s.env.GetDB().Drop(s.ctx))
}

func (s *systemMetricsConnectorSuite) TestFindSystemMetricsByTypeFound() {
	// no execution
	opts := dbModel.SystemMetricsFindOptions{
		TaskID:         "task1",
		EmptyExecution: true,
	}
	r, err := s.sc.FindSystemMetricsByType(s.ctx, "uptime", opts)
	s.Require().NoError(err)
	data, err := ioutil.ReadAll(r)
	s.Require().NoError(err)
	s.Equal("execution 1", string(data))

	// execution specified
	opts.Execution = 0
	opts.EmptyExecution = false
	r, err = s.sc.FindSystemMetricsByType(s.ctx, "uptime", opts)
	s.Require().NoError(err)
	data, err = ioutil.ReadAll(r)
	s.Require().NoError(err)
	s.Equal("execution 0", string(data))
}

func (s *systemMetricsConnectorSuite) TestFindSystemMetricsByTypeNotFound() {
	// task id DNE
	opts := dbModel.SystemMetricsFindOptions{
		TaskID:         "DNE",
		EmptyExecution: true,
	}
	r, err := s.sc.FindSystemMetricsByType(s.ctx, "uptime", opts)
	s.Error(err)
	s.Nil(r)

	// execution DNE
	opts = dbModel.SystemMetricsFindOptions{
		TaskID:    "task1",
		Execution: 5,
	}
	r, err = s.sc.FindSystemMetricsByType(s.ctx, "uptime", opts)
	s.Error(err)
	s.Nil(r)

	// metric type DNE
	opts.TaskID = "task1"
	r, err = s.sc.FindSystemMetricsByType(s.ctx, "DNE", opts)
	s.Error(err)
	s.Nil(r)
}
