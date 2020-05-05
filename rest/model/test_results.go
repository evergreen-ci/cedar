package model

import (
	dbmodel "github.com/evergreen-ci/cedar/model"
	"github.com/pkg/errors"
)

// APITestResult describes a single test result.
type APITestResult struct {
	TaskID         APIString `json:"task_id"`
	Execution      int       `json:"execution"`
	TestName       APIString `json:"test_name"`
	Trial          int       `json:"trial"`
	Status         APIString `json:"status"`
	LogURL         APIString `json:"log_url"`
	LineNum        int       `json:"line_num"`
	TaskCreateTime APITime   `json:"task_create_time"`
	TestStartTime  APITime   `json:"test_start_time"`
	TestEndTime    APITime   `json:"test_end_time"`
}

// Import transforms a TestResult object into an APITestResult object.
func (a *APITestResult) Import(i interface{}) error {
	switch tr := i.(type) {
	case dbmodel.TestResult:
		a.TaskID = ToAPIString(tr.TaskID)
		a.Execution = tr.Execution
		a.TestName = ToAPIString(tr.TestName)
		a.Trial = tr.Trial
		a.Status = ToAPIString(tr.Status)
		a.LogURL = ToAPIString(tr.LogURL)
		a.LineNum = tr.LineNum
		a.TaskCreateTime = NewTime(tr.TaskCreateTime)
		a.TestStartTime = NewTime(tr.TestStartTime)
		a.TestEndTime = NewTime(tr.TestEndTime)
	default:
		return errors.New("incorrect type when converting to APITestResult type")
	}
	return nil
}
