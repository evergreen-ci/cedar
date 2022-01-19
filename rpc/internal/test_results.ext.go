package internal

import (
	"regexp"
	"strings"

	"github.com/evergreen-ci/cedar/model"
	"github.com/pkg/errors"
)

// Export exports TestResultsInfo to the corresponding TestResultsInfo type in
// the model package.
func (t *TestResultsInfo) Export() (model.TestResultsInfo, error) {
	ignore := true
	if !t.HistoricalDataDisabled {
		taskName := t.DisplayTaskName
		if taskName == "" {
			taskName = t.TaskName
		}

		var err error
		ignore, err = historicalTestDataIgnore(t.HistoricalDataIgnore, taskName)
		if err != nil {
			return model.TestResultsInfo{}, errors.Wrap(err, "checking if should calculate historical test data")
		}
	}

	return model.TestResultsInfo{
		Project:                t.Project,
		Version:                t.Version,
		Variant:                t.Variant,
		TaskName:               t.TaskName,
		DisplayTaskName:        t.DisplayTaskName,
		TaskID:                 t.TaskId,
		DisplayTaskID:          t.DisplayTaskId,
		Execution:              int(t.Execution),
		RequestType:            t.RequestType,
		HistoricalDataDisabled: ignore,
		Mainline:               t.Mainline,
	}, nil
}

// historicalTestDataIgnore checks whether the given task name matches any of
// the patterns in the ignore slice.
func historicalTestDataIgnore(ignore []string, taskName string) (bool, error) {
	for _, pattern := range ignore {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false, errors.Wrapf(err, "compiling regexp '%s'", pattern)
		}
		if re.MatchString(taskName) {
			return true, nil
		}
	}

	return false, nil
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
		LogTestName:     t.LogTestName,
		LogURL:          t.LogUrl,
		RawLogURL:       t.RawLogUrl,
		LineNum:         int(t.LineNum),
		TaskCreateTime:  t.TaskCreateTime.AsTime(),
		TestStartTime:   t.TestStartTime.AsTime(),
		TestEndTime:     t.TestEndTime.AsTime(),
	}
}
