package model

import (
	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/utility"
	"github.com/pkg/errors"
)

// APITestResults describes a set of test results and related information.
type APITestResults struct {
	Stats   APITestResultsStats `json:"stats"`
	Results []APITestResult     `json:"results"`
}

// APITestResult describes a single test result.
type APITestResult struct {
	TaskID          *string `json:"task_id"`
	Execution       int     `json:"execution"`
	TestName        *string `json:"test_name"`
	DisplayTestName *string `json:"display_test_name,omitempty"`
	GroupID         *string `json:"group_id,omitempty"`
	Trial           int     `json:"trial,omitempty"`
	Status          *string `json:"status"`
	BaseStatus      *string `json:"base_status,omitempty"`
	LogTestName     *string `json:"log_test_name,omitempty"`
	LogURL          *string `json:"log_url,omitempty"`
	RawLogURL       *string `json:"raw_log_url,omitempty"`
	LineNum         int     `json:"line_num"`
	TaskCreateTime  APITime `json:"task_create_time"`
	TestStartTime   APITime `json:"test_start_time"`
	TestEndTime     APITime `json:"test_end_time"`
}

// Import transforms a TestResult object into an APITestResult object.
func (a *APITestResult) Import(i interface{}) error {
	switch tr := i.(type) {
	case dbModel.TestResult:
		a.TaskID = utility.ToStringPtr(tr.TaskID)
		a.Execution = tr.Execution
		a.TestName = utility.ToStringPtr(tr.TestName)
		if tr.DisplayTestName != "" {
			a.DisplayTestName = utility.ToStringPtr(tr.DisplayTestName)
		}
		if tr.GroupID != "" {
			a.GroupID = utility.ToStringPtr(tr.GroupID)
		}
		a.Trial = tr.Trial
		a.Status = utility.ToStringPtr(tr.Status)
		if tr.BaseStatus != "" {
			a.BaseStatus = utility.ToStringPtr(tr.BaseStatus)
		}
		if tr.LogTestName != "" {
			a.LogTestName = utility.ToStringPtr(tr.LogTestName)
		}
		if tr.LogURL != "" {
			a.LogURL = utility.ToStringPtr(tr.LogURL)
		}
		if tr.RawLogURL != "" {
			a.RawLogURL = utility.ToStringPtr(tr.RawLogURL)
		}
		a.LineNum = tr.LineNum
		a.TaskCreateTime = NewTime(tr.TaskCreateTime)
		a.TestStartTime = NewTime(tr.TestStartTime)
		a.TestEndTime = NewTime(tr.TestEndTime)
	default:
		return errors.New("incorrect type when converting to APITestResult type")
	}

	return nil
}

// APITTestResultsStats describes basic stats for a group of test results.
type APITestResultsStats struct {
	TotalCount    int  `json:"total_count"`
	FailedCount   int  `json:"failed_count"`
	FilteredCount *int `json:"filtered_count,omitempty"`
}

// Import transforms a TestResultsStats object into an APITestResultsStats
// object.
func (a *APITestResultsStats) Import(i interface{}) error {
	switch stats := i.(type) {
	case dbModel.TestResultsStats:
		a.TotalCount = stats.TotalCount
		a.FailedCount = stats.FailedCount
	case int:
		a.FilteredCount = utility.ToIntPtr(stats)
	default:
		return errors.New("incorrect type when converting to APITTestResultsStats type")
	}

	return nil
}
