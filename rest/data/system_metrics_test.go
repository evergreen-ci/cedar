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
		s.Require().NoError(systemMetrics.Append(s.ctx, "uptime", dbModel.FileText, []byte(fmt.Sprintf("execution %d\n", taskInfo.Execution))))
		s.Require().NoError(systemMetrics.Append(s.ctx, "uptime", dbModel.FileText, []byte("chunk2")))
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
	findOpts := dbModel.SystemMetricsFindOptions{
		TaskID:         "task1",
		EmptyExecution: true,
	}
	downloadOpts := dbModel.SystemMetricsDownloadOptions{MetricType: "uptime"}
	data, nextIdx, err := s.sc.FindSystemMetricsByType(s.ctx, findOpts, downloadOpts)
	s.Require().NoError(err)
	s.Equal("execution 1\nchunk2", string(data))
	s.Equal(2, nextIdx)

	// execution specified
	findOpts.Execution = 0
	findOpts.EmptyExecution = false
	data, nextIdx, err = s.sc.FindSystemMetricsByType(s.ctx, findOpts, downloadOpts)
	s.Require().NoError(err)
	s.Equal("execution 0\nchunk2", string(data))
	s.Equal(2, nextIdx)

	// paginated
	downloadOpts.PageSize = 5
	data, nextIdx, err = s.sc.FindSystemMetricsByType(s.ctx, findOpts, downloadOpts)
	s.Require().NoError(err)
	s.Equal("execution 0\n", string(data))
	s.Equal(1, nextIdx)

	downloadOpts.StartIndex = 1
	data, nextIdx, err = s.sc.FindSystemMetricsByType(s.ctx, findOpts, downloadOpts)
	s.Require().NoError(err)
	s.Equal("chunk2", string(data))
	s.Equal(2, nextIdx)

	downloadOpts.StartIndex = 2
	data, nextIdx, err = s.sc.FindSystemMetricsByType(s.ctx, findOpts, downloadOpts)
	s.Require().NoError(err)
	s.Nil(data)
	s.Equal(2, nextIdx)
}

func (s *systemMetricsConnectorSuite) TestFindSystemMetricsByTypeNotFound() {
	// task id DNE
	findOpts := dbModel.SystemMetricsFindOptions{
		TaskID:         "DNE",
		EmptyExecution: true,
	}
	downloadOpts := dbModel.SystemMetricsDownloadOptions{MetricType: "uptime"}
	_, _, err := s.sc.FindSystemMetricsByType(s.ctx, findOpts, downloadOpts)
	s.Error(err)

	// execution DNE
	findOpts = dbModel.SystemMetricsFindOptions{
		TaskID:    "task1",
		Execution: 5,
	}
	_, _, err = s.sc.FindSystemMetricsByType(s.ctx, findOpts, downloadOpts)
	s.Error(err)

	// metric type DNE
	findOpts.TaskID = "task1"
	downloadOpts.MetricType = "DNE"
	_, _, err = s.sc.FindSystemMetricsByType(s.ctx, findOpts, downloadOpts)
	s.Error(err)
}
