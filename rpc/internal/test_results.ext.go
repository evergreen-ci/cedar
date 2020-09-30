package internal

import (
	"github.com/evergreen-ci/cedar/model"
	"github.com/golang/protobuf/ptypes"
	"github.com/pkg/errors"
)

// Export exports TestResultsInfo to the corresponding TestResultsInfo type in
// the model package.
func (t TestResultsInfo) Export() model.TestResultsInfo {
	return model.TestResultsInfo{
		Project:         t.Project,
		Version:         t.Version,
		Variant:         t.Variant,
		TaskName:        t.TaskName,
		DisplayTaskName: t.DisplayTaskName,
		TaskID:          t.TaskId,
		Execution:       int(t.Execution),
		RequestType:     t.RequestType,
		Mainline:        t.Mainline,
	}
}

// Export exports TestResult to the corresponding TestResult type in the model
// package.
func (t TestResult) Export() (model.TestResult, error) {
	taskCreateTime, err := ptypes.Timestamp(t.TaskCreateTime)
	if err != nil {
		return model.TestResult{}, errors.Wrap(err, "problem converting task create time timestamp")
	}
	testStartTime, err := ptypes.Timestamp(t.TestStartTime)
	if err != nil {
		return model.TestResult{}, errors.Wrap(err, "problem converting test start time timestamp")
	}
	testEndTime, err := ptypes.Timestamp(t.TestEndTime)
	if err != nil {
		return model.TestResult{}, errors.Wrap(err, "problem converting test end time timestamp")
	}

	return model.TestResult{
		TestName:       t.TestName,
		Trial:          int(t.Trial),
		Status:         t.Status,
		LineNum:        int(t.LineNum),
		TaskCreateTime: taskCreateTime,
		TestStartTime:  testStartTime,
		TestEndTime:    testEndTime,
	}, nil
}
