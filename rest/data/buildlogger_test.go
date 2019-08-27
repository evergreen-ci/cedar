package data

import (
	"context"
	"testing"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
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
	s.sc = &MockConnector{CachedLogs: s.logs}
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
	}
	for _, logInfo := range logs {
		log := model.CreateLog(logInfo, model.PailS3)
		log.Setup(s.env)
		s.Require().NoError(log.SaveNew(s.ctx))
		s.logs[log.ID] = *log
	}
}

func (s *buildloggerConnectorSuite) TearDownSuite() {
	defer s.cancel()
	s.NoError(s.env.GetDB().Drop(s.ctx))
}

func (s *buildloggerConnectorSuite) TestFindLogByIdExists() {
	for id := range s.logs {
		l, err := s.sc.FindLogById(s.ctx, id)
		s.Require().NoError(err)
		s.Equal(id, *l.ID)
	}
}

func (s *buildloggerConnectorSuite) TestFindLogByIdDNE() {
	l, err := s.sc.FindLogById(s.ctx, "DNE")
	s.Error(err)
	s.Nil(l)
}
