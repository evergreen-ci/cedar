package internal

import (
	"testing"

	"github.com/evergreen-ci/utility"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestTestResultsInfoExport(t *testing.T) {
	info := TestResultsInfo{
		Project:         "project",
		Version:         "version",
		Variant:         "variant",
		TaskName:        "task_name",
		DisplayTaskName: "display_task_name",
		TaskId:          "task_id",
		DisplayTaskId:   "display_task_id",
		Execution:       1,
		RequestType:     "request_type",
		Mainline:        true,
	}

	modelInfo, err := info.Export()
	require.NoError(t, err)
	assert.Equal(t, info.Project, modelInfo.Project)
	assert.Equal(t, info.Version, modelInfo.Version)
	assert.Equal(t, info.Variant, modelInfo.Variant)
	assert.Equal(t, info.TaskName, modelInfo.TaskName)
	assert.Equal(t, info.DisplayTaskName, modelInfo.DisplayTaskName)
	assert.Equal(t, info.TaskId, modelInfo.TaskID)
	assert.Equal(t, int(info.Execution), modelInfo.Execution)
	assert.Equal(t, info.RequestType, modelInfo.RequestType)
}

func TestTestResultExport(t *testing.T) {
	result := TestResult{
		TestName: "test_name",
		GroupId:  "group",
		Trial:    1,
		Status:   "status",
		LogInfo: &TestLogInfo{
			LogName:       "log0",
			LogsToMerge:   []string{"log1", "log2"},
			LineNum:       100,
			RenderingType: utility.ToStringPtr("resmoke"),
			Version:       1,
		},
		LogTestName:    "log_test_name",
		LogUrl:         "log_url",
		RawLogUrl:      "raw_log_url",
		LineNum:        1000,
		TaskCreateTime: &timestamppb.Timestamp{Seconds: 1588278536},
		TestStartTime:  &timestamppb.Timestamp{Seconds: 1588278500},
		TestEndTime:    &timestamppb.Timestamp{Seconds: 1588278490},
	}

	t.Run("PopulatedLogInfo", func(t *testing.T) {
		modelResult := result.Export()

		assert.Equal(t, result.TestName, modelResult.TestName)
		assert.Equal(t, result.DisplayTestName, modelResult.DisplayTestName)
		assert.Equal(t, int(result.Trial), modelResult.Trial)
		assert.Equal(t, result.Status, modelResult.Status)
		assert.Equal(t, result.LogInfo.LogName, modelResult.LogInfo.LogName)
		assert.Equal(t, result.LogInfo.LogsToMerge, modelResult.LogInfo.LogsToMerge)
		assert.Equal(t, result.LogInfo.LineNum, modelResult.LogInfo.LineNum)
		assert.Equal(t, result.LogInfo.RenderingType, modelResult.LogInfo.RenderingType)
		assert.Equal(t, result.LogInfo.Version, modelResult.LogInfo.Version)
		assert.Equal(t, result.GroupId, modelResult.GroupID)
		assert.Equal(t, result.LogTestName, modelResult.LogTestName)
		assert.Equal(t, result.LogUrl, modelResult.LogURL)
		assert.Equal(t, result.RawLogUrl, modelResult.RawLogURL)
		assert.Equal(t, int(result.LineNum), modelResult.LineNum)
		assert.Equal(t, result.TaskCreateTime.AsTime(), modelResult.TaskCreateTime)
		assert.Equal(t, result.TestStartTime.AsTime(), modelResult.TestStartTime)
		assert.Equal(t, result.TestEndTime.AsTime(), modelResult.TestEndTime)
	})
	t.Run("EmptyLogInfo", func(t *testing.T) {
		result.LogInfo = nil
		modelResult := result.Export()
		assert.Nil(t, modelResult.LogInfo)
	})
}
