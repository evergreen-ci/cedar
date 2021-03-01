package model

import (
	"strconv"
	"strings"

	dbmodel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/utility"
	"github.com/pkg/errors"
)

// APITestResult describes a single test result.
type APITestResult struct {
	TaskID         *string `json:"task_id"`
	Execution      int     `json:"execution"`
	TestName       *string `json:"test_name"`
	Trial          int     `json:"trial"`
	Status         *string `json:"status"`
	LogURL         *string `json:"log_url"`
	LineNum        int     `json:"line_num"`
	TaskCreateTime APITime `json:"task_create_time"`
	TestStartTime  APITime `json:"test_start_time"`
	TestEndTime    APITime `json:"test_end_time"`
}

// Import transforms a TestResult object into an APITestResult object.
func (a *APITestResult) Import(i interface{}) error {
	switch tr := i.(type) {
	case dbmodel.TestResult:
		a.TaskID = utility.ToStringPtr(tr.TaskID)
		a.Execution = tr.Execution
		a.TestName = utility.ToStringPtr(tr.TestName)
		a.Trial = tr.Trial
		a.Status = utility.ToStringPtr(tr.Status)
		a.LineNum = tr.LineNum
		a.TaskCreateTime = NewTime(tr.TaskCreateTime)
		a.TestStartTime = NewTime(tr.TestStartTime)
		a.TestEndTime = NewTime(tr.TestEndTime)

		if tr.LogURL == "" {
			a.LogURL = utility.ToStringPtr(strings.Join([]string{
				"https://cedar.mongodb.com",
				"rest",
				"v1",
				"buildlogger",
				"test_name",
				tr.TaskID,
				tr.TestName + "?execution=" + strconv.Itoa(
					tr.Execution,
				),
			}, "/"))
		} else {
			a.LogURL = utility.ToStringPtr(tr.LogURL)
		}
	default:
		return errors.New("incorrect type when converting to APITestResult type")
	}
	return nil
}
