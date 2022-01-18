package internal

import (
	"testing"

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

	t.Run("HistoricalTestDataEnabled", func(t *testing.T) {
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
		assert.False(t, modelInfo.HistoricalDataDisabled)
		assert.Equal(t, info.Mainline, modelInfo.Mainline)
	})
	t.Run("HistoricalTestDataDisabled", func(t *testing.T) {
		info.HistoricalDataDisabled = true

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
		assert.True(t, modelInfo.HistoricalDataDisabled)
		assert.Equal(t, info.Mainline, modelInfo.Mainline)

		info.HistoricalDataDisabled = false
	})
	t.Run("HistoricalTestDataIgnored", func(t *testing.T) {
		t.Run("DisplayTaskName", func(t *testing.T) {
			info.HistoricalDataIgnore = []string{"display_task_name"}

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
			assert.True(t, modelInfo.HistoricalDataDisabled)
			assert.Equal(t, info.Mainline, modelInfo.Mainline)

			info.HistoricalDataIgnore = nil
		})
		t.Run("TaskName", func(t *testing.T) {
			displayTaskName := info.DisplayTaskName
			info.DisplayTaskName = ""
			info.HistoricalDataIgnore = []string{"task_name"}

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
			assert.True(t, modelInfo.HistoricalDataDisabled)
			assert.Equal(t, info.Mainline, modelInfo.Mainline)

			info.DisplayTaskName = displayTaskName
			info.HistoricalDataIgnore = nil
		})
	})
}

func TestTestResultExport(t *testing.T) {
	result := TestResult{
		TestName:       "test_name",
		GroupId:        "group",
		Trial:          1,
		Status:         "status",
		LogTestName:    "log_test_name",
		LogUrl:         "log_url",
		RawLogUrl:      "raw_log_url",
		LineNum:        1000,
		TaskCreateTime: &timestamppb.Timestamp{Seconds: 1588278536},
		TestStartTime:  &timestamppb.Timestamp{Seconds: 1588278500},
		TestEndTime:    &timestamppb.Timestamp{Seconds: 1588278490},
	}

	modelResult := result.Export()
	assert.Equal(t, result.TestName, modelResult.TestName)
	assert.Equal(t, result.DisplayTestName, modelResult.DisplayTestName)
	assert.Equal(t, int(result.Trial), modelResult.Trial)
	assert.Equal(t, result.Status, modelResult.Status)
	assert.Equal(t, result.GroupId, modelResult.GroupID)
	assert.Equal(t, result.LogTestName, modelResult.LogTestName)
	assert.Equal(t, result.LogUrl, modelResult.LogURL)
	assert.Equal(t, result.RawLogUrl, modelResult.RawLogURL)
	assert.Equal(t, int(result.LineNum), modelResult.LineNum)
	assert.Equal(t, result.TaskCreateTime.AsTime(), modelResult.TaskCreateTime)
	assert.Equal(t, result.TestStartTime.AsTime(), modelResult.TestStartTime)
	assert.Equal(t, result.TestEndTime.AsTime(), modelResult.TestEndTime)
}
