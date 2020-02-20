package data

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/testutils"
	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/pail"
	"github.com/stretchr/testify/suite"
)

type buildloggerConnectorSuite struct {
	ctx     context.Context
	cancel  context.CancelFunc
	sc      Connector
	env     cedar.Environment
	logs    map[string]model.Log
	tempDir string

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
	s.sc = &MockConnector{
		CachedLogs: s.logs,
		env:        cedar.GetEnvironment(),
		Bucket:     s.tempDir,
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

	// setup config
	var err error
	s.tempDir, err = ioutil.TempDir(".", "buildlogger_connector")
	s.Require().NoError(err)
	conf := model.NewCedarConfig(s.env)
	conf.Bucket = model.BucketConfig{BuildLogsBucket: s.tempDir}
	s.Require().NoError(conf.Save())

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

		opts := pail.LocalOptions{
			Path:   s.tempDir,
			Prefix: log.Artifact.Prefix,
		}
		bucket, err := pail.NewLocalBucket(opts)
		s.Require().NoError(err)
		chunks, _, err := testutils.CreateLog(s.ctx, bucket, 100, 10)
		log.Artifact.Chunks = chunks

		log.Setup(s.env)
		s.Require().NoError(log.SaveNew(s.ctx))
		s.logs[log.ID] = *log
		time.Sleep(time.Second)
	}
}

func (s *buildloggerConnectorSuite) TearDownSuite() {
	defer s.cancel()
	s.NoError(os.RemoveAll(s.tempDir))
	s.NoError(s.env.GetDB().Drop(s.ctx))
}

func (s *buildloggerConnectorSuite) TestFindLogByIDExists() {
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
				SoftSizeLimit: 500,
			}
			data, next, paginated, err := s.sc.FindLogByID(s.ctx, findOpts)
			s.Require().NoError(err)
			s.True(paginated)
			it, err := log.Download(s.ctx, findOpts.TimeRange)
			s.Require().NoError(err)
			readerOpts := model.LogIteratorReaderOptions{
				PrintTime:     printTime,
				PrintPriority: !printTime,
				SoftSizeLimit: findOpts.SoftSizeLimit,
			}
			expected, err := ioutil.ReadAll(model.NewLogIteratorReader(s.ctx, it, readerOpts))
			s.Require().NoError(err)
			s.Equal(expected, data)
			s.Equal(it.Item().Timestamp, next)

			l, err := s.sc.FindLogMetadataByID(s.ctx, id)
			s.Require().NoError(err)
			s.Equal(id, *l.ID)

			// limit
			findOpts.Limit = 50
			readerOpts.Limit = 100
			readerOpts.SoftSizeLimit = 0
			data, next, paginated, err = s.sc.FindLogByID(s.ctx, findOpts)
			s.Require().NoError(err)
			s.False(paginated)
			it, err = log.Download(s.ctx, findOpts.TimeRange)
			s.Require().NoError(err)
			expected, err = ioutil.ReadAll(model.NewLogIteratorReader(s.ctx, it, readerOpts))
			s.Require().NoError(err)
			s.Equal(expected, data)
			s.Equal(it.Item().Timestamp, next)
		}
	}
}

func (s *buildloggerConnectorSuite) TestFindLogByIDDNE() {
	_, _, _, err := s.sc.FindLogByID(s.ctx, BuildloggerOptions{ID: "DNE"})
	s.Error(err)

	l, err := s.sc.FindLogMetadataByID(s.ctx, "DNE")
	s.Error(err)
	s.Nil(l)
}

func (s *buildloggerConnectorSuite) TestFindLogsByTaskIDExists() {
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

		findOpts := BuildloggerOptions{
			TaskID:        opts.Info.TaskID,
			Execution:     1,
			TimeRange:     opts.TimeRange,
			PrintTime:     printTime,
			PrintPriority: !printTime,
			SoftSizeLimit: 500,
		}
		data, next, paginated, err := s.sc.FindLogsByTaskID(s.ctx, findOpts)
		s.Require().NoError(err)
		s.True(paginated)
		it, err := logs.Merge(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(it)
		readerOpts := model.LogIteratorReaderOptions{
			PrintTime:     printTime,
			PrintPriority: !printTime,
			SoftSizeLimit: 500,
		}
		expected, err := ioutil.ReadAll(model.NewLogIteratorReader(s.ctx, it, readerOpts))
		s.Require().NoError(err)
		s.Equal(expected, data)
		s.Equal(it.Item().Timestamp, next)

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
		it, err = logs.Merge(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(it)

		findOpts.Tags = opts.Info.Tags
		data, next, paginated, err = s.sc.FindLogsByTaskID(s.ctx, findOpts)
		s.Require().NoError(err)
		expected, err = ioutil.ReadAll(model.NewLogIteratorReader(s.ctx, it, readerOpts))
		s.Require().NoError(err)
		s.Equal(expected, data)
		s.Equal(it.Item().Timestamp, next)

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
		it, err = logs.Merge(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(it)

		findOpts.ProcessName = opts.Info.ProcessName
		data, next, paginated, err = s.sc.FindLogsByTaskID(s.ctx, findOpts)
		s.Require().NoError(err)
		expected, err = ioutil.ReadAll(model.NewLogIteratorReader(s.ctx, it, readerOpts))
		s.Require().NoError(err)
		s.Equal(expected, data)
		s.Equal(it.Item().Timestamp, next)

		// limit
		logs = model.Logs{}
		logs.Setup(s.env)
		s.Require().NoError(logs.Find(s.ctx, opts))
		it, err = logs.Merge(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(it)

		findOpts.Limit = 100
		data, next, paginated, err = s.sc.FindLogsByTaskID(s.ctx, findOpts)
		s.Require().NoError(err)
		s.False(paginated)
		readerOpts.Limit = 100
		readerOpts.SoftSizeLimit = 0
		expected, err = ioutil.ReadAll(model.NewLogIteratorReader(s.ctx, it, readerOpts))
		s.Require().NoError(err)
		s.Equal(expected, data)
		s.Equal(it.Item().Timestamp, next)

		// tail and execution
		opts.Info.Execution = 0
		opts.Empty.Execution = true
		logs = model.Logs{}
		logs.Setup(s.env)
		s.Require().NoError(logs.Find(s.ctx, opts))
		it, err = logs.Merge(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(it)

		findOpts.Execution = 0
		findOpts.Tail = 100
		data, next, paginated, err = s.sc.FindLogsByTaskID(s.ctx, findOpts)
		s.Require().NoError(err)
		readerOpts.Limit = 0
		readerOpts.TailN = findOpts.Tail
		expected, err = ioutil.ReadAll(model.NewLogIteratorReader(s.ctx, it, readerOpts))
		s.Require().NoError(err)
		s.Equal(expected, data)
		s.Equal(it.Item().Timestamp, next)
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
	_, _, _, err := s.sc.FindLogsByTaskID(s.ctx, opts)
	s.Error(err)

	apiLogs, err := s.sc.FindLogMetadataByTaskID(s.ctx, BuildloggerOptions{TaskID: "DNE"})
	s.Error(err)
	s.Nil(apiLogs)
}

func (s *buildloggerConnectorSuite) TestFindLogsByTestNameExists() {
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
		it, err := logs.Merge(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(it)

		findOpts := BuildloggerOptions{
			TaskID:        opts.Info.TaskID,
			TestName:      opts.Info.TestName,
			TimeRange:     opts.TimeRange,
			PrintTime:     printTime,
			PrintPriority: !printTime,
			SoftSizeLimit: 500,
		}
		data, next, paginated, err := s.sc.FindLogsByTestName(s.ctx, findOpts)
		s.Require().NoError(err)
		s.True(paginated)
		readerOpts := model.LogIteratorReaderOptions{
			PrintTime:     printTime,
			PrintPriority: !printTime,
			SoftSizeLimit: findOpts.SoftSizeLimit,
		}
		expected, err := ioutil.ReadAll(model.NewLogIteratorReader(s.ctx, it, readerOpts))
		s.Require().NoError(err)
		s.Equal(expected, data)
		s.Equal(it.Item().Timestamp, next)

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
		it, err = logs.Merge(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(it)

		findOpts.Tags = opts.Info.Tags
		data, next, paginated, err = s.sc.FindLogsByTestName(s.ctx, findOpts)
		s.Require().NoError(err)
		s.True(paginated)
		expected, err = ioutil.ReadAll(model.NewLogIteratorReader(s.ctx, it, readerOpts))
		s.Require().NoError(err)
		s.Equal(expected, data)
		s.Equal(it.Item().Timestamp, next)

		apiLogs, err = s.sc.FindLogMetadataByTestName(s.ctx, findOpts)
		s.Require().NoError(err)
		s.Len(apiLogs, 1)
		for _, apiLog := range apiLogs {
			s.Equal(opts.Info.TaskID, *apiLog.Info.TaskID)
			s.Equal(opts.Info.TestName, *apiLog.Info.TestName)
		}

		// limit
		it, err = logs.Merge(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(it)

		findOpts.Limit = 100
		data, next, paginated, err = s.sc.FindLogsByTestName(s.ctx, findOpts)
		s.Require().NoError(err)
		s.False(paginated)
		readerOpts.Limit = findOpts.Limit
		readerOpts.SoftSizeLimit = 0
		expected, err = ioutil.ReadAll(model.NewLogIteratorReader(s.ctx, it, readerOpts))
		s.Require().NoError(err)
		s.Equal(expected, data)
		s.Equal(it.Item().Timestamp, next)
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
	it, err := logs.Merge(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(it)

	findOpts := BuildloggerOptions{
		TaskID:        opts.Info.TaskID,
		TimeRange:     opts.TimeRange,
		SoftSizeLimit: 500,
	}
	data, next, paginated, err := s.sc.FindLogsByTestName(s.ctx, findOpts)
	s.Require().NoError(err)
	s.True(paginated)
	readerOpts := model.LogIteratorReaderOptions{SoftSizeLimit: findOpts.SoftSizeLimit}
	expected, err := ioutil.ReadAll(model.NewLogIteratorReader(s.ctx, it, readerOpts))
	s.Require().NoError(err)
	s.Equal(expected, data)
	s.Equal(it.Item().Timestamp, next)

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
	it, err = logs.Merge(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(it)

	findOpts.Tags = opts.Info.Tags
	data, next, paginated, err = s.sc.FindLogsByTestName(s.ctx, findOpts)
	s.Require().NoError(err)
	s.True(paginated)
	expected, err = ioutil.ReadAll(model.NewLogIteratorReader(s.ctx, it, readerOpts))
	s.Require().NoError(err)
	s.Equal(expected, data)
	s.Equal(it.Item().Timestamp, next)

	apiLogs, err = s.sc.FindLogMetadataByTestName(s.ctx, findOpts)
	s.Require().NoError(err)
	s.Len(apiLogs, 1)
	for _, apiLog := range apiLogs {
		s.Equal(opts.Info.TaskID, *apiLog.Info.TaskID)
		s.Equal(opts.Info.TestName, *apiLog.Info.TestName)
	}

	// limit
	it, err = logs.Merge(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(it)

	findOpts.Limit = 100
	data, next, paginated, err = s.sc.FindLogsByTestName(s.ctx, findOpts)
	s.Require().NoError(err)
	s.False(paginated)
	readerOpts.Limit = findOpts.Limit
	readerOpts.SoftSizeLimit = 0
	expected, err = ioutil.ReadAll(model.NewLogIteratorReader(s.ctx, it, readerOpts))
	s.Require().NoError(err)
	s.Equal(expected, data)
	s.Equal(it.Item().Timestamp, next)
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
	_, _, _, err := s.sc.FindLogsByTestName(s.ctx, findOpts)
	s.Error(err)

	apiLogs, err := s.sc.FindLogMetadataByTestName(s.ctx, findOpts)
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
		logs1 := model.Logs{}
		logs1.Setup(s.env)
		s.Require().NoError(logs1.Find(s.ctx, opts))
		it1, err := logs1.Merge(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(it1)

		opts.Info = model.LogInfo{
			TaskID: "task1",
			Tags:   []string{"tag1"},
		}
		opts.Empty = model.EmptyLogInfo{TestName: true}
		logs2 := model.Logs{}
		logs2.Setup(s.env)
		s.Require().NoError(logs2.Find(s.ctx, opts))
		it2, err := logs2.Merge(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(it2)
		it := model.NewMergingIterator(it1, it2)

		findOpts := BuildloggerOptions{
			TaskID:        opts.Info.TaskID,
			TestName:      "test0",
			Tags:          opts.Info.Tags,
			TimeRange:     opts.TimeRange,
			PrintTime:     printTime,
			PrintPriority: !printTime,
			SoftSizeLimit: 500,
		}
		data, next, paginated, err := s.sc.FindGroupedLogs(s.ctx, findOpts)
		s.Require().NoError(err)
		s.True(paginated)
		readerOpts := model.LogIteratorReaderOptions{
			PrintTime:     printTime,
			PrintPriority: !printTime,
			SoftSizeLimit: findOpts.SoftSizeLimit,
		}
		expected, err := ioutil.ReadAll(model.NewLogIteratorReader(s.ctx, it, readerOpts))
		s.Require().NoError(err)
		s.Equal(expected, data)
		s.Equal(it.Item().Timestamp, next)

		// limit
		it1, err = logs1.Merge(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(it1)
		it2, err = logs2.Merge(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(it2)
		it = model.NewMergingIterator(it1, it2)

		findOpts.Limit = 100
		data, next, paginated, err = s.sc.FindGroupedLogs(s.ctx, findOpts)
		s.Require().NoError(err)
		readerOpts.Limit = findOpts.Limit
		readerOpts.SoftSizeLimit = 0
		expected, err = ioutil.ReadAll(model.NewLogIteratorReader(s.ctx, it, readerOpts))
		s.Require().NoError(err)
		s.Equal(expected, data)
		s.Equal(it.Item().Timestamp, next)
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
		it, err := logs.Merge(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(it)
		it = model.NewMergingIterator(it)

		findOpts := BuildloggerOptions{
			TaskID:        opts.Info.TaskID,
			TestName:      opts.Info.TestName,
			Tags:          []string{"tag4"},
			TimeRange:     opts.TimeRange,
			PrintTime:     printTime,
			PrintPriority: !printTime,
			SoftSizeLimit: 500,
		}
		data, next, paginated, err := s.sc.FindGroupedLogs(s.ctx, findOpts)
		s.Require().NoError(err)
		s.True(paginated)
		readerOpts := model.LogIteratorReaderOptions{
			PrintTime:     printTime,
			PrintPriority: !printTime,
			SoftSizeLimit: findOpts.SoftSizeLimit,
		}
		expected, err := ioutil.ReadAll(model.NewLogIteratorReader(s.ctx, it, readerOpts))
		s.Require().NoError(err)
		s.Equal(expected, data)
		s.Equal(it.Item().Timestamp, next)

		// limit
		it, err = logs.Merge(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(it)
		it = model.NewMergingIterator(it)

		findOpts.Limit = 100
		data, next, paginated, err = s.sc.FindGroupedLogs(s.ctx, findOpts)
		s.Require().NoError(err)
		s.False(paginated)
		readerOpts.Limit = findOpts.Limit
		readerOpts.SoftSizeLimit = 0
		expected, err = ioutil.ReadAll(model.NewLogIteratorReader(s.ctx, it, readerOpts))
		s.Require().NoError(err)
		s.Equal(expected, data)
		s.Equal(it.Item().Timestamp, next)
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
	_, _, _, err := s.sc.FindGroupedLogs(s.ctx, findOpts)

	s.Error(err)
}
