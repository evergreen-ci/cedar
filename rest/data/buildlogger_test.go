package data

import (
	"context"
	"fmt"
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
			Tags:        []string{"tag1", "tag2", "tag3"},
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
			Tags:        []string{"tag1", "tag2"},
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
			TestName:    "",
			ProcessName: "mongod0",
			Format:      model.LogFormatText,
			Tags:        []string{"tag1", "tag2", "tag3"},
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
			TestName:    "",
			ProcessName: "mongod1",
			Format:      model.LogFormatText,
			Tags:        []string{"tag1", "tag2"},
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
		{
			Project:     "test",
			Version:     "0",
			Variant:     "linux",
			TaskName:    "task0",
			TaskID:      "task2",
			Execution:   1,
			TestName:    "test1",
			ProcessName: "mongod0",
			Format:      model.LogFormatText,
			Tags:        []string{"tag4"},
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
		time.Sleep(time.Second)
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

func (s *buildloggerConnectorSuite) TestFindLogByIDExists() {
	tr := util.TimeRange{
		StartAt: time.Now().Add(-time.Hour),
		EndAt:   time.Now(),
	}
	for id, log := range s.logs {
		for _, printTime := range []bool{true, false} {
			r, err := s.sc.FindLogByID(s.ctx, id, tr, printTime)
			s.Require().NoError(err)
			expectedIt, err := log.Download(s.ctx, tr)
			s.Require().NoError(err)
			opts := model.LogIteratorReaderOptions{PrintTime: printTime}
			s.Equal(model.NewLogIteratorReader(s.ctx, expectedIt, opts), r)

			l, err := s.sc.FindLogMetadataByID(s.ctx, id)
			s.Require().NoError(err)
			s.Equal(id, *l.ID)
		}
	}
}

func (s *buildloggerConnectorSuite) TestFindLogByIDDNE() {
	r, err := s.sc.FindLogByID(s.ctx, "DNE", util.TimeRange{}, false)
	s.Error(err)
	s.Nil(r)

	l, err := s.sc.FindLogMetadataByID(s.ctx, "DNE")
	s.Error(err)
	s.Nil(l)
}

func (s *buildloggerConnectorSuite) TestFindLogsByTaskIDExists() {
	for _, printTime := range []bool{true, false} {
		readerOpts := model.LogIteratorReaderOptions{PrintTime: printTime}

		opts := model.LogFindOptions{
			TimeRange: util.TimeRange{
				StartAt: time.Now().Add(-time.Hour),
				EndAt:   time.Now(),
			},
			Info: model.LogInfo{TaskID: "task1"},
		}
		logs := model.Logs{}
		logs.Setup(s.env)
		s.Require().NoError(logs.Find(s.ctx, opts))
		expectedIt, err := logs.Merge(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(expectedIt)

		r, err := s.sc.FindLogsByTaskID(s.ctx, opts.Info.TaskID, opts.TimeRange, 0, printTime)
		s.Require().NoError(err)
		s.Equal(model.NewLogIteratorReader(s.ctx, expectedIt, readerOpts), r)

		apiLogs, err := s.sc.FindLogMetadataByTaskID(s.ctx, opts.Info.TaskID)
		s.Require().NoError(err)
		s.Len(apiLogs, 4)
		for _, apiLog := range apiLogs {
			s.Equal(opts.Info.TaskID, *apiLog.Info.TaskID)
		}

		// with tags
		opts.Info.Tags = []string{"tag3"}
		logs = model.Logs{}
		logs.Setup(s.env)
		s.Require().NoError(logs.Find(s.ctx, opts))
		expectedIt, err = logs.Merge(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(expectedIt)

		r, err = s.sc.FindLogsByTaskID(s.ctx, opts.Info.TaskID, opts.TimeRange, 0, printTime, opts.Info.Tags...)
		s.Require().NoError(err)
		s.Equal(model.NewLogIteratorReader(s.ctx, expectedIt, readerOpts), r)

		apiLogs, err = s.sc.FindLogMetadataByTaskID(s.ctx, opts.Info.TaskID, opts.Info.Tags...)
		s.Require().NoError(err)
		s.Len(apiLogs, 2)
		for _, apiLog := range apiLogs {
			s.Equal(opts.Info.TaskID, *apiLog.Info.TaskID)
		}

		// tail
		logs = model.Logs{}
		logs.Setup(s.env)
		s.Require().NoError(logs.Find(s.ctx, opts))
		fmt.Println(logs.Logs)
		expectedIt, err = logs.Merge(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(expectedIt)

		r, err = s.sc.FindLogsByTaskID(s.ctx, opts.Info.TaskID, opts.TimeRange, 100, printTime, opts.Info.Tags...)
		s.Require().NoError(err)
		readerOpts.TailN = 100
		s.Equal(model.NewLogIteratorReader(s.ctx, expectedIt, readerOpts), r)
	}
}

func (s *buildloggerConnectorSuite) TestFindLogsByTaskIDDNE() {
	tr := util.TimeRange{
		StartAt: time.Now().Add(-time.Hour),
		EndAt:   time.Now(),
	}

	r, err := s.sc.FindLogsByTaskID(s.ctx, "DNE", tr, 0, false)
	s.Error(err)
	s.Nil(r)

	apiLogs, err := s.sc.FindLogMetadataByTaskID(s.ctx, "DNE")
	s.Error(err)
	s.Nil(apiLogs)
}

func (s *buildloggerConnectorSuite) TestFindLogsByTestNameExists() {
	for _, printTime := range []bool{true, false} {
		readerOpts := model.LogIteratorReaderOptions{PrintTime: printTime}

		opts := model.LogFindOptions{
			TimeRange: util.TimeRange{
				StartAt: time.Now().Add(-time.Hour),
				EndAt:   time.Now(),
			},
			Info: model.LogInfo{
				TaskID:   "task1",
				TestName: "test0",
			},
		}
		logs := model.Logs{}
		logs.Setup(s.env)
		s.Require().NoError(logs.Find(s.ctx, opts))
		expectedIt, err := logs.Merge(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(expectedIt)

		r, err := s.sc.FindLogsByTestName(s.ctx, opts.Info.TaskID, opts.Info.TestName, opts.TimeRange, printTime)
		s.Require().NoError(err)
		s.Equal(model.NewLogIteratorReader(s.ctx, expectedIt, readerOpts), r)

		apiLogs, err := s.sc.FindLogMetadataByTestName(s.ctx, opts.Info.TaskID, opts.Info.TestName)
		s.Require().NoError(err)
		s.Len(apiLogs, 2)
		for _, apiLog := range apiLogs {
			s.Equal(opts.Info.TaskID, *apiLog.Info.TaskID)
			s.Equal(opts.Info.TestName, *apiLog.Info.TestName)
		}

		// with tags
		opts.Info.Tags = []string{"tag3"}
		logs = model.Logs{}
		logs.Setup(s.env)
		s.Require().NoError(logs.Find(s.ctx, opts))
		expectedIt, err = logs.Merge(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(expectedIt)

		r, err = s.sc.FindLogsByTestName(s.ctx, opts.Info.TaskID, opts.Info.TestName, opts.TimeRange, printTime, opts.Info.Tags...)
		s.Require().NoError(err)
		s.Equal(model.NewLogIteratorReader(s.ctx, expectedIt, readerOpts), r)

		apiLogs, err = s.sc.FindLogMetadataByTestName(s.ctx, opts.Info.TaskID, opts.Info.TestName, opts.Info.Tags...)
		s.Require().NoError(err)
		s.Len(apiLogs, 1)
		for _, apiLog := range apiLogs {
			s.Equal(opts.Info.TaskID, *apiLog.Info.TaskID)
			s.Equal(opts.Info.TestName, *apiLog.Info.TestName)
		}
	}
}

func (s *buildloggerConnectorSuite) TestFindLogsByTestNameEmpty() {
	opts := model.LogFindOptions{
		TimeRange: util.TimeRange{
			StartAt: time.Now().Add(-time.Hour),
			EndAt:   time.Now(),
		},
		Info:  model.LogInfo{TaskID: "task1"},
		Empty: model.EmptyLogInfo{TestName: true},
	}
	logs := model.Logs{}
	logs.Setup(s.env)
	s.Require().NoError(logs.Find(s.ctx, opts))
	expectedIt, err := logs.Merge(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(expectedIt)

	r, err := s.sc.FindLogsByTestName(s.ctx, opts.Info.TaskID, "", opts.TimeRange, false)
	s.Require().NoError(err)
	s.Equal(model.NewLogIteratorReader(s.ctx, expectedIt, model.LogIteratorReaderOptions{}), r)

	apiLogs, err := s.sc.FindLogMetadataByTestName(s.ctx, opts.Info.TaskID, "")
	s.Require().NoError(err)
	s.Len(apiLogs, 2)
	for _, apiLog := range apiLogs {
		s.Equal(opts.Info.TaskID, *apiLog.Info.TaskID)
		s.Equal(opts.Info.TestName, *apiLog.Info.TestName)
	}

	// with tags
	opts.Info.Tags = []string{"tag3"}
	logs = model.Logs{}
	logs.Setup(s.env)
	s.Require().NoError(logs.Find(s.ctx, opts))
	expectedIt, err = logs.Merge(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(expectedIt)

	r, err = s.sc.FindLogsByTestName(s.ctx, opts.Info.TaskID, "", opts.TimeRange, false, opts.Info.Tags...)
	s.Require().NoError(err)
	s.Equal(model.NewLogIteratorReader(s.ctx, expectedIt, model.LogIteratorReaderOptions{}), r)

	apiLogs, err = s.sc.FindLogMetadataByTestName(s.ctx, opts.Info.TaskID, "", opts.Info.Tags...)
	s.Require().NoError(err)
	s.Len(apiLogs, 1)
	for _, apiLog := range apiLogs {
		s.Equal(opts.Info.TaskID, *apiLog.Info.TaskID)
		s.Equal(opts.Info.TestName, *apiLog.Info.TestName)
	}
}

func (s *buildloggerConnectorSuite) TestFindLogsByTestNameDNE() {
	tr := util.TimeRange{
		StartAt: time.Now().Add(-time.Hour),
		EndAt:   time.Now(),
	}

	r, err := s.sc.FindLogsByTestName(s.ctx, "task1", "DNE", tr, false)
	s.Error(err)
	s.Nil(r)

	apiLogs, err := s.sc.FindLogMetadataByTestName(s.ctx, "task1", "DNE")
	s.Error(err)
	s.Nil(apiLogs)
}

func (s *buildloggerConnectorSuite) TestFindGroupedLogsExists() {
	for _, printTime := range []bool{true, false} {
		opts := model.LogFindOptions{
			TimeRange: util.TimeRange{
				StartAt: time.Now().Add(-time.Hour),
				EndAt:   time.Now(),
			},
			Info: model.LogInfo{
				TaskID:   "task1",
				TestName: "test0",
				Tags:     []string{"tag1"},
			},
		}
		logs := model.Logs{}
		logs.Setup(s.env)
		s.Require().NoError(logs.Find(s.ctx, opts))
		expectedIt1, err := logs.Merge(s.ctx)
		fmt.Println(logs.Logs)
		s.Require().NoError(err)
		s.Require().NotNil(expectedIt1)

		opts.Info = model.LogInfo{
			TaskID: "task1",
			Tags:   []string{"tag1"},
		}
		opts.Empty = model.EmptyLogInfo{TestName: true}
		logs = model.Logs{}
		logs.Setup(s.env)
		s.Require().NoError(logs.Find(s.ctx, opts))
		fmt.Println(logs.Logs)
		expectedIt2, err := logs.Merge(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(expectedIt2)

		r, err := s.sc.FindGroupedLogs(s.ctx, opts.Info.TaskID, "test0", "tag1", opts.TimeRange, printTime)
		s.Require().NoError(err)
		readerOpts := model.LogIteratorReaderOptions{PrintTime: printTime}
		s.Equal(model.NewLogIteratorReader(s.ctx, model.NewMergingIterator(expectedIt1, expectedIt2), readerOpts), r)
	}
}

func (s *buildloggerConnectorSuite) TestFindGroupedLogsOnlyTestLevel() {
	for _, printTime := range []bool{true, false} {
		opts := model.LogFindOptions{
			TimeRange: util.TimeRange{
				StartAt: time.Now().Add(-time.Hour),
				EndAt:   time.Now(),
			},
			Info: model.LogInfo{
				TaskID:   "task2",
				TestName: "test1",
			},
		}
		logs := model.Logs{}
		logs.Setup(s.env)
		s.Require().NoError(logs.Find(s.ctx, opts))
		expectedIt, err := logs.Merge(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(expectedIt)

		r, err := s.sc.FindGroupedLogs(s.ctx, opts.Info.TaskID, opts.Info.TestName, "tag4", opts.TimeRange, printTime)
		s.Require().NoError(err)

		readerOpts := model.LogIteratorReaderOptions{PrintTime: printTime}
		s.Equal(model.NewLogIteratorReader(s.ctx, model.NewMergingIterator(expectedIt), readerOpts), r)
	}
}

func (s *buildloggerConnectorSuite) TestFindGroupedLogsDNE() {
	tr := util.TimeRange{
		StartAt: time.Now().Add(-time.Hour),
		EndAt:   time.Now(),
	}
	r, err := s.sc.FindGroupedLogs(s.ctx, "task1", "DNE", "tag1", tr, false)

	s.Error(err)
	s.Nil(r)
}
