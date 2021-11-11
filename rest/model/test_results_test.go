package model

import (
	"testing"
	"time"

	dbmodel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/utility"
	"github.com/stretchr/testify/assert"
)

func TestTestResultImport(t *testing.T) {
	t.Run("InvalidType", func(t *testing.T) {
		apiTestResult := &APITestResult{}
		assert.Error(t, apiTestResult.Import(dbmodel.TestResults{}))
	})
	t.Run("ValidTestResult", func(t *testing.T) {
		tr := dbmodel.TestResult{
			TaskID:          "task_id",
			GroupID:         "group_id",
			Execution:       2,
			TestName:        "test_name",
			DisplayTestName: "display",
			Trial:           3,
			Status:          "pass",
			BaseStatus:      "pass",
			LogTestName:     "log",
			LogURL:          "url",
			RawLogURL:       "raw_url",
			LineNum:         102,
			TaskCreateTime:  time.Now().Add(-time.Hour),
			TestStartTime:   time.Now().Add(-30 * time.Minute),
			TestEndTime:     time.Now(),
		}
		expected := &APITestResult{
			TaskID:          utility.ToStringPtr(tr.TaskID),
			GroupID:         utility.ToStringPtr(tr.GroupID),
			Execution:       tr.Execution,
			TestName:        utility.ToStringPtr(tr.TestName),
			DisplayTestName: utility.ToStringPtr(tr.DisplayTestName),
			Trial:           tr.Trial,
			Status:          utility.ToStringPtr(tr.Status),
			BaseStatus:      utility.ToStringPtr(tr.BaseStatus),
			LogTestName:     utility.ToStringPtr(tr.LogTestName),
			LogURL:          utility.ToStringPtr(tr.LogURL),
			RawLogURL:       utility.ToStringPtr(tr.RawLogURL),
			LineNum:         tr.LineNum,
			TaskCreateTime:  NewTime(tr.TaskCreateTime),
			TestStartTime:   NewTime(tr.TestStartTime),
			TestEndTime:     NewTime(tr.TestEndTime),
		}
		apiTestResult := &APITestResult{}
		assert.NoError(t, apiTestResult.Import(tr))
		assert.Equal(t, expected, apiTestResult)
	})
}
