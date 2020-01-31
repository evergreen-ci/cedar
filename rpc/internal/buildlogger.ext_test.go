package internal

import (
	"testing"
	"time"

	"github.com/evergreen-ci/cedar/model"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/mongodb/grip/level"
	"github.com/stretchr/testify/assert"
)

func TestLogFormatExport(t *testing.T) {
	for _, test := range []struct {
		name                string
		rpcFormat           LogFormat
		expectedModelFormat model.LogFormat
	}{
		{
			name:                "Uknown",
			rpcFormat:           LogFormat_LOG_FORMAT_UNKNOWN,
			expectedModelFormat: model.LogFormatUnknown,
		},
		{
			name:                "Text",
			rpcFormat:           LogFormat_LOG_FORMAT_TEXT,
			expectedModelFormat: model.LogFormatText,
		},
		{
			name:                "JSON",
			rpcFormat:           LogFormat_LOG_FORMAT_JSON,
			expectedModelFormat: model.LogFormatJSON,
		},
		{
			name:                "BSON",
			rpcFormat:           LogFormat_LOG_FORMAT_BSON,
			expectedModelFormat: model.LogFormatBSON,
		},
		{
			name:                "Default",
			rpcFormat:           LogFormat(4),
			expectedModelFormat: model.LogFormatUnknown,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expectedModelFormat, test.rpcFormat.Export())
		})
	}
}

func TestLogStorageExport(t *testing.T) {
	for _, test := range []struct {
		name             string
		storageType      LogStorage
		expectedPailType model.PailType
	}{
		{
			name:             "S3",
			storageType:      LogStorage_LOG_STORAGE_S3,
			expectedPailType: model.PailS3,
		},
		{
			name:             "GridFS",
			storageType:      LogStorage_LOG_STORAGE_GRIDFS,
			expectedPailType: model.PailGridFS,
		},
		{
			name:             "Local",
			storageType:      LogStorage_LOG_STORAGE_LOCAL,
			expectedPailType: model.PailLocal,
		},
		{
			name:             "Default",
			storageType:      LogStorage(3),
			expectedPailType: model.PailLocal,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expectedPailType, test.storageType.Export())
		})
	}
}

func TestLogLineExport(t *testing.T) {
	t.Run("ValidTimestamp", func(t *testing.T) {
		logLine := LogLine{
			Priority:  90,
			Timestamp: &timestamp.Timestamp{Seconds: 253402300799},
			Data:      "Goodbye year 9999\n",
		}
		modelLogLine, err := logLine.Export()
		assert.NoError(t, err)
		assert.Equal(t, modelLogLine.Priority, level.Alert)
		assert.Equal(t, time.Date(9999, time.December, 31, 23, 59, 59, 0, time.UTC), modelLogLine.Timestamp)
		assert.Equal(t, logLine.Data, modelLogLine.Data)
	})
	t.Run("InvalidTimestamp", func(t *testing.T) {
		logLine := LogLine{
			Priority:  30,
			Timestamp: &timestamp.Timestamp{Seconds: 253402300800},
			Data:      "Hello year 10000\n",
		}
		modelLogLine, err := logLine.Export()
		assert.Error(t, err)
		assert.Zero(t, modelLogLine)
	})
}

func TestLogInfoExport(t *testing.T) {
	logInfo := LogInfo{
		Project:   "project",
		Version:   "version",
		Variant:   "variant",
		TaskName:  "task_name",
		TaskId:    "task_id",
		Execution: 3,
		TestName:  "test_name",
		Trial:     2,
		ProcName:  "process_name",
		Format:    LogFormat_LOG_FORMAT_TEXT,
		Tags:      []string{"tag1", "tag2", "tag3"},
		Arguments: map[string]string{"hello": "world", "goodbye": "world"},
		Mainline:  true,
	}
	modelLogInfo := logInfo.Export()
	assert.Equal(t, logInfo.Project, modelLogInfo.Project)
	assert.Equal(t, logInfo.Version, modelLogInfo.Version)
	assert.Equal(t, logInfo.Variant, modelLogInfo.Variant)
	assert.Equal(t, logInfo.TaskName, modelLogInfo.TaskName)
	assert.Equal(t, logInfo.TaskId, modelLogInfo.TaskID)
	assert.Equal(t, int(logInfo.Execution), modelLogInfo.Execution)
	assert.Equal(t, logInfo.TestName, modelLogInfo.TestName)
	assert.Equal(t, int(logInfo.Trial), modelLogInfo.Trial)
	assert.Equal(t, logInfo.ProcName, modelLogInfo.ProcessName)
	assert.Equal(t, logInfo.Format.Export(), modelLogInfo.Format)
	assert.Equal(t, logInfo.Arguments, modelLogInfo.Arguments)
	assert.Equal(t, logInfo.Mainline, modelLogInfo.Mainline)

}
