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
	"github.com/evergreen-ci/gimlet"
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
			Execution:   0,
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
	responder := gimlet.NewResponseBuilder()
	s.Require().NoError(responder.SetFormat(gimlet.TEXT))

	for id, log := range s.logs {
		for _, printTime := range []bool{true, false} {
			findOpts := BuildloggerOptions{
				ID: id,
				TimeRange: util.TimeRange{
					StartAt: time.Now().Add(-time.Hour),
					EndAt:   time.Now(),
				},
				PrintTime:     printTime,
				PrintPriority: !printTime,
			}
			r, err := s.sc.FindLogByID(s.ctx, findOpts)
			s.Require().NoError(err)
			expectedIt, err := log.Download(s.ctx, findOpts.TimeRange)
			s.Require().NoError(err)
			expected := &buildloggerPaginatedResponder{
				ctx: s.ctx,
				it:  expectedIt,
				tr:  findOpts.TimeRange,
				readerOpts: model.LogIteratorReaderOptions{
					PrintTime:     printTime,
					PrintPriority: !printTime,
				},
				Responder: responder,
			}
			s.Equal(expected, r)

			l, err := s.sc.FindLogMetadataByID(s.ctx, id)
			s.Require().NoError(err)
			s.Equal(id, *l.ID)

			// limit
			findOpts.Limit = 100
			r, err = s.sc.FindLogByID(s.ctx, findOpts)
			s.Require().NoError(err)
			expected.readerOpts.Limit = findOpts.Limit
			s.Equal(gimlet.NewTextResponse(model.NewLogIteratorReader(s.ctx, expectedIt, expected.readerOpts)), r)
		}
	}
}

func (s *buildloggerConnectorSuite) TestFindLogByIDDNE() {
	r, err := s.sc.FindLogByID(s.ctx, BuildloggerOptions{ID: "DNE"})
	s.Error(err)
	s.Nil(r)

	l, err := s.sc.FindLogMetadataByID(s.ctx, "DNE")
	s.Error(err)
	s.Nil(l)
}

func (s *buildloggerConnectorSuite) TestFindLogsByTaskIDExists() {
	responder := gimlet.NewResponseBuilder()
	s.Require().NoError(responder.SetFormat(gimlet.TEXT))

	for _, printTime := range []bool{true, false} {
		opts := model.LogFindOptions{
			TimeRange: util.TimeRange{
				StartAt: time.Now().Add(-time.Hour),
				EndAt:   time.Now(),
			},
			Info: model.LogInfo{
				TaskID:    "task1",
				Execution: 1,
			},
		}
		logs := model.Logs{}
		logs.Setup(s.env)
		s.Require().NoError(logs.Find(s.ctx, opts))
		expectedIt, err := logs.Merge(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(expectedIt)

		findOpts := BuildloggerOptions{
			TaskID:        opts.Info.TaskID,
			Execution:     1,
			TimeRange:     opts.TimeRange,
			PrintTime:     printTime,
			PrintPriority: !printTime,
		}
		r, err := s.sc.FindLogsByTaskID(s.ctx, findOpts)
		s.Require().NoError(err)
		expected := &buildloggerPaginatedResponder{
			ctx: s.ctx,
			it:  expectedIt,
			tr:  findOpts.TimeRange,
			readerOpts: model.LogIteratorReaderOptions{
				PrintTime:     printTime,
				PrintPriority: !printTime,
			},
			Responder: responder,
		}
		s.Equal(expected, r)

		apiLogs, err := s.sc.FindLogMetadataByTaskID(s.ctx, findOpts)
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

		findOpts.Tags = opts.Info.Tags
		r, err = s.sc.FindLogsByTaskID(s.ctx, findOpts)
		s.Require().NoError(err)
		expected.it = expectedIt
		s.Equal(expected, r)

		apiLogs, err = s.sc.FindLogMetadataByTaskID(s.ctx, findOpts)
		s.Require().NoError(err)
		s.Len(apiLogs, 2)
		for _, apiLog := range apiLogs {
			s.Equal(opts.Info.TaskID, *apiLog.Info.TaskID)
		}

		// with process name
		opts.Info.ProcessName = "mongod0"
		logs = model.Logs{}
		logs.Setup(s.env)
		s.Require().NoError(logs.Find(s.ctx, opts))
		expectedIt, err = logs.Merge(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(expectedIt)

		findOpts.ProcessName = opts.Info.ProcessName
		r, err = s.sc.FindLogsByTaskID(s.ctx, findOpts)
		s.Require().NoError(err)
		expected.it = expectedIt
		s.Equal(expected, r)

		// limit
		findOpts.Limit = 100
		r, err = s.sc.FindLogsByTaskID(s.ctx, findOpts)
		s.Require().NoError(err)
		expected.readerOpts.Limit = findOpts.Limit
		s.Equal(gimlet.NewTextResponse(model.NewLogIteratorReader(s.ctx, expectedIt, expected.readerOpts)), r)

		// tail
		findOpts.Execution = 0
		opts.Info.Execution = 0
		opts.Empty.Execution = true
		logs = model.Logs{}
		logs.Setup(s.env)
		s.Require().NoError(logs.Find(s.ctx, opts))
		fmt.Println(logs.Logs)
		expectedIt, err = logs.Merge(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(expectedIt)

		findOpts.Tail = 100
		r, err = s.sc.FindLogsByTaskID(s.ctx, findOpts)
		s.Require().NoError(err)
		expected.readerOpts.TailN = findOpts.Tail
		s.Equal(gimlet.NewTextResponse(model.NewLogIteratorReader(s.ctx, expectedIt, expected.readerOpts)), r)
	}
}

func (s *buildloggerConnectorSuite) TestFindLogsByTaskIDDNE() {
	opts := BuildloggerOptions{
		TaskID: "DNE",
		TimeRange: util.TimeRange{
			StartAt: time.Now().Add(-time.Hour),
			EndAt:   time.Now(),
		},
	}
	r, err := s.sc.FindLogsByTaskID(s.ctx, opts)
	s.Error(err)
	s.Nil(r)

	apiLogs, err := s.sc.FindLogMetadataByTaskID(s.ctx, BuildloggerOptions{TaskID: "DNE"})
	s.Error(err)
	s.Nil(apiLogs)
}

func (s *buildloggerConnectorSuite) TestFindLogsByTestNameExists() {
	responder := gimlet.NewResponseBuilder()
	s.Require().NoError(responder.SetFormat(gimlet.TEXT))

	for _, printTime := range []bool{true, false} {

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

		findOpts := BuildloggerOptions{
			TaskID:        opts.Info.TaskID,
			TestName:      opts.Info.TestName,
			TimeRange:     opts.TimeRange,
			PrintTime:     printTime,
			PrintPriority: !printTime,
		}
		r, err := s.sc.FindLogsByTestName(s.ctx, findOpts)
		s.Require().NoError(err)
		expected := &buildloggerPaginatedResponder{
			ctx: s.ctx,
			it:  expectedIt,
			tr:  findOpts.TimeRange,
			readerOpts: model.LogIteratorReaderOptions{
				PrintTime:     printTime,
				PrintPriority: !printTime,
			},
			Responder: responder,
		}
		s.Equal(expected, r)

		apiLogs, err := s.sc.FindLogMetadataByTestName(s.ctx, findOpts)
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

		findOpts.Tags = opts.Info.Tags
		r, err = s.sc.FindLogsByTestName(s.ctx, findOpts)
		s.Require().NoError(err)
		expected.it = expectedIt
		s.Equal(expected, r)

		apiLogs, err = s.sc.FindLogMetadataByTestName(s.ctx, findOpts)
		s.Require().NoError(err)
		s.Len(apiLogs, 1)
		for _, apiLog := range apiLogs {
			s.Equal(opts.Info.TaskID, *apiLog.Info.TaskID)
			s.Equal(opts.Info.TestName, *apiLog.Info.TestName)
		}

		// limit
		findOpts.Limit = 100
		r, err = s.sc.FindLogsByTestName(s.ctx, findOpts)
		s.Require().NoError(err)
		expected.readerOpts.Limit = findOpts.Limit
		s.Equal(gimlet.NewTextResponse(model.NewLogIteratorReader(s.ctx, expectedIt, expected.readerOpts)), r)
	}
}

func (s *buildloggerConnectorSuite) TestFindLogsByTestNameEmpty() {
	responder := gimlet.NewResponseBuilder()
	s.Require().NoError(responder.SetFormat(gimlet.TEXT))

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

	findOpts := BuildloggerOptions{
		TaskID:    opts.Info.TaskID,
		TimeRange: opts.TimeRange,
	}
	r, err := s.sc.FindLogsByTestName(s.ctx, findOpts)
	s.Require().NoError(err)
	expected := &buildloggerPaginatedResponder{
		ctx:        s.ctx,
		it:         expectedIt,
		tr:         findOpts.TimeRange,
		readerOpts: model.LogIteratorReaderOptions{},
		Responder:  responder,
	}
	s.Equal(expected, r)

	apiLogs, err := s.sc.FindLogMetadataByTestName(s.ctx, findOpts)
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

	findOpts.Tags = opts.Info.Tags
	r, err = s.sc.FindLogsByTestName(s.ctx, findOpts)
	s.Require().NoError(err)
	expected.it = expectedIt
	s.Equal(expected, r)

	apiLogs, err = s.sc.FindLogMetadataByTestName(s.ctx, findOpts)
	s.Require().NoError(err)
	s.Len(apiLogs, 1)
	for _, apiLog := range apiLogs {
		s.Equal(opts.Info.TaskID, *apiLog.Info.TaskID)
		s.Equal(opts.Info.TestName, *apiLog.Info.TestName)
	}

	// limit
	findOpts.Limit = 100
	r, err = s.sc.FindLogsByTestName(s.ctx, findOpts)
	s.Require().NoError(err)
	expected.readerOpts.Limit = findOpts.Limit
	s.Equal(gimlet.NewTextResponse(model.NewLogIteratorReader(s.ctx, expectedIt, expected.readerOpts)), r)
}

func (s *buildloggerConnectorSuite) TestFindLogsByTestNameDNE() {
	findOpts := BuildloggerOptions{
		TaskID:   "task1",
		TestName: "DNE",
		TimeRange: util.TimeRange{
			StartAt: time.Now().Add(-time.Hour),
			EndAt:   time.Now(),
		},
	}
	r, err := s.sc.FindLogsByTestName(s.ctx, findOpts)
	s.Error(err)
	s.Nil(r)

	apiLogs, err := s.sc.FindLogMetadataByTestName(s.ctx, findOpts)
	s.Error(err)
	s.Nil(apiLogs)
}

func (s *buildloggerConnectorSuite) TestFindGroupedLogsExists() {
	responder := gimlet.NewResponseBuilder()
	s.Require().NoError(responder.SetFormat(gimlet.TEXT))

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

		findOpts := BuildloggerOptions{
			TaskID:        opts.Info.TaskID,
			TestName:      "test0",
			Tags:          opts.Info.Tags,
			TimeRange:     opts.TimeRange,
			PrintTime:     printTime,
			PrintPriority: !printTime,
		}
		r, err := s.sc.FindGroupedLogs(s.ctx, findOpts)
		s.Require().NoError(err)
		expected := &buildloggerPaginatedResponder{
			ctx: s.ctx,
			it:  model.NewMergingIterator(expectedIt1, expectedIt2),
			tr:  findOpts.TimeRange,
			readerOpts: model.LogIteratorReaderOptions{
				PrintTime:     printTime,
				PrintPriority: !printTime,
			},
			Responder: responder,
		}
		s.Equal(expected, r)

		// limit
		findOpts.Limit = 100
		r, err = s.sc.FindGroupedLogs(s.ctx, findOpts)
		s.Require().NoError(err)
		expected.readerOpts.Limit = findOpts.Limit
		s.Equal(gimlet.NewTextResponse(model.NewLogIteratorReader(s.ctx, expected.it, expected.readerOpts)), r)
	}
}

func (s *buildloggerConnectorSuite) TestFindGroupedLogsOnlyTestLevel() {
	responder := gimlet.NewResponseBuilder()
	s.Require().NoError(responder.SetFormat(gimlet.TEXT))

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

		findOpts := BuildloggerOptions{
			TaskID:        opts.Info.TaskID,
			TestName:      opts.Info.TestName,
			Tags:          []string{"tag4"},
			TimeRange:     opts.TimeRange,
			PrintTime:     printTime,
			PrintPriority: !printTime,
		}
		r, err := s.sc.FindGroupedLogs(s.ctx, findOpts)
		s.Require().NoError(err)
		expected := &buildloggerPaginatedResponder{
			ctx: s.ctx,
			it:  model.NewMergingIterator(expectedIt),
			tr:  findOpts.TimeRange,
			readerOpts: model.LogIteratorReaderOptions{
				PrintTime:     printTime,
				PrintPriority: !printTime,
			},
			Responder: responder,
		}
		s.Equal(expected, r)

		// limit
		findOpts.Limit = 100
		r, err = s.sc.FindGroupedLogs(s.ctx, findOpts)
		s.Require().NoError(err)
		expected.readerOpts.Limit = findOpts.Limit
		s.Equal(gimlet.NewTextResponse(model.NewLogIteratorReader(s.ctx, expected.it, expected.readerOpts)), r)
	}
}

func (s *buildloggerConnectorSuite) TestFindGroupedLogsDNE() {
	findOpts := BuildloggerOptions{
		TaskID:   "task1",
		TestName: "DNE",
		Tags:     []string{"tag1"},
		TimeRange: util.TimeRange{
			StartAt: time.Now().Add(-time.Hour),
			EndAt:   time.Now(),
		},
	}
	r, err := s.sc.FindGroupedLogs(s.ctx, findOpts)

	s.Error(err)
	s.Nil(r)
}
