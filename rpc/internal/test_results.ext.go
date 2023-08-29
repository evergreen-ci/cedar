package internal

import (
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/utility"
)

// Export exports TestResultsInfo to the corresponding TestResultsInfo type in
// the model package.
func (t *TestResultsInfo) Export() (model.TestResultsInfo, error) {
	return model.TestResultsInfo{
		Project:         t.Project,
		Version:         t.Version,
		Variant:         t.Variant,
		TaskName:        t.TaskName,
		DisplayTaskName: t.DisplayTaskName,
		TaskID:          t.TaskId,
		DisplayTaskID:   t.DisplayTaskId,
		Execution:       int(t.Execution),
		RequestType:     t.RequestType,
		Mainline:        t.Mainline,
	}, nil
}

// Export exports TestResult to the corresponding TestResult type in the model
// package.
func (t *TestResult) Export() model.TestResult {
	return model.TestResult{
		TestName:        t.TestName,
		DisplayTestName: t.DisplayTestName,
		GroupID:         t.GroupId,
		Trial:           int(t.Trial),
		Status:          t.Status,
		LogInfo:         t.LogInfo.Export(),
		LogTestName:     t.LogTestName,
		LogURL:          t.LogUrl,
		RawLogURL:       t.RawLogUrl,
		LineNum:         int(t.LineNum),
		TaskCreateTime:  t.TaskCreateTime.AsTime(),
		TestStartTime:   t.TestStartTime.AsTime(),
		TestEndTime:     t.TestEndTime.AsTime(),
	}
}

// Export exports TestLogInfo to the corresponding TestLogInfo type in the
// model package.
func (t *TestLogInfo) Export() *model.TestLogInfo {
	if t == nil {
		return nil
	}
	var logsToMerge []*string
	for _, logName := range t.LogsToMerge {
		logsToMerge = append(logsToMerge, utility.ToStringPtr(logName))
	}

	return &model.TestLogInfo{
		LogName:       t.LogName,
		LogsToMerge:   logsToMerge,
		LineNum:       t.LineNum,
		RenderingType: t.RenderingType,
		Version:       t.Version,
	}
}
