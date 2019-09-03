package data

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/util"
	"github.com/stretchr/testify/suite"
)

type buildloggerConnectorSuite struct {
	ctx    context.Context
	cancel context.CancelFunc
	sc     Connector
	env    cedar.Environment
	logs   map[string]model.Log

	suite.Suite
}

func TestBuildloggerConnectorSuiteDB(t *testing.T) {
	s := new(buildloggerConnectorSuite)
	s.setup()
	s.sc = CreateNewDBConnector(s.env)
	suite.Run(t, s)
}

func TestBuildloggerConnectorSuiteMock(t *testing.T) {
	s := new(buildloggerConnectorSuite)
	s.setup()
	wd, err := os.Getwd()
	s.Require().NoError(err)
	s.sc = &MockConnector{
		CachedLogs: s.logs,
		env:        cedar.GetEnvironment(),
		Bucket:     wd,
	}
	suite.Run(t, s)
}

func (s *buildloggerConnectorSuite) setup() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.env = cedar.GetEnvironment()
	s.Require().NotNil(s.env)
	db := s.env.GetDB()
	s.Require().NotNil(db)
	s.logs = map[string]model.Log{}

	logs := []model.LogInfo{
		{
			Project:     "test",
			Version:     "0",
			Variant:     "linux",
			TaskName:    "task0",
			TaskID:      "task1",
			Execution:   1,
			TestName:    "test0",
			ProcessName: "mongod0",
			Format:      model.LogFormatText,
			Arguments:   map[string]string{"arg1": "val1", "arg2": "val2"},
			ExitCode:    1,
			Mainline:    true,
		},
		{
			Project:     "test",
			Version:     "0",
			Variant:     "linux",
			TaskName:    "task0",
			TaskID:      "task1",
			Execution:   1,
			TestName:    "test0",
			ProcessName: "mongod1",
			Format:      model.LogFormatText,
			Arguments:   map[string]string{"arg1": "val1", "arg2": "val2"},
			ExitCode:    1,
			Mainline:    true,
		},
		{
			Project:     "test",
			Version:     "0",
			Variant:     "linux",
			TaskName:    "task0",
			TaskID:      "task2",
			Execution:   1,
			TestName:    "test0",
			ProcessName: "mongod0",
			Format:      model.LogFormatText,
			Arguments:   map[string]string{"arg1": "val1", "arg2": "val2"},
			ExitCode:    0,
			Mainline:    true,
		},
	}
	for _, logInfo := range logs {
		log := model.CreateLog(logInfo, model.PailLocal)
		log.Setup(s.env)
		s.Require().NoError(log.SaveNew(s.ctx))
		s.logs[log.ID] = *log
	}

	// setup config
	wd, err := os.Getwd()
	s.Require().NoError(err)
	conf := model.NewCedarConfig(s.env)
	conf.Bucket = model.BucketConfig{BuildLogsBucket: wd}
	s.Require().NoError(conf.Save())
}

func (s *buildloggerConnectorSuite) TearDownSuite() {
	defer s.cancel()
	s.NoError(s.env.GetDB().Drop(s.ctx))
}

func (s *buildloggerConnectorSuite) TestFindLogByIdExists() {
	tr := util.TimeRange{
		StartAt: time.Now().Add(-time.Hour),
		EndAt:   time.Now(),
	}
	for id, log := range s.logs {
		it, err := s.sc.FindLogById(s.ctx, id, tr)
		s.Require().NoError(err)
		expectedIt, err := log.Download(s.ctx, tr)
		s.Require().NoError(err)
		s.Equal(expectedIt, it)

		l, err := s.sc.FindLogMetadataById(s.ctx, id)
		s.Require().NoError(err)
		s.Equal(id, *l.ID)
	}
}

func (s *buildloggerConnectorSuite) TestFindLogByIdDNE() {
	it, err := s.sc.FindLogById(s.ctx, "DNE", util.TimeRange{})
	s.Error(err)
	s.Nil(it)

	l, err := s.sc.FindLogMetadataById(s.ctx, "DNE")
	s.Error(err)
	s.Nil(l)
}

func (s *buildloggerConnectorSuite) TestFindLogsByTaskIdExists() {
	for i := 0; i < 2; i++ {
		opts := model.LogFindOptions{
			TimeRange: util.TimeRange{
				StartAt: time.Now().Add(-time.Hour),
				EndAt:   time.Now(),
			},
			Info:  model.LogInfo{TaskID: "task1"},
			Limit: i,
		}
		logs := model.Logs{}
		logs.Setup(s.env)
		s.Require().NoError(logs.Find(s.ctx, opts))
		expectedIt, err := logs.Merge(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(expectedIt)

		it, err := s.sc.FindLogsByTaskId(s.ctx, opts.Info.TaskID, opts.TimeRange, opts.Limit)
		s.Require().NoError(err)
		s.Equal(expectedIt, it)

		apiLogs, err := s.sc.FindLogMetadataByTaskId(s.ctx, opts.Info.TaskID, opts.Limit)
		s.Require().NoError(err)
		s.Require().Len(apiLogs, len(logs.Logs))
		for i, log := range logs.Logs {
			s.Equal(log.ID, *apiLogs[i].ID)
		}
	}
}

func (s *buildloggerConnectorSuite) TestFindLogsByTaskIdDNE() {
	tr := util.TimeRange{
		StartAt: time.Now().Add(-time.Hour),
		EndAt:   time.Now(),
	}

	it, err := s.sc.FindLogsByTaskId(s.ctx, "DNE", tr, 0)
	s.Error(err)
	s.Nil(it)

	apiLogs, err := s.sc.FindLogMetadataByTaskId(s.ctx, "DNE", 0)
	s.Error(err)
	s.Nil(apiLogs)
}
