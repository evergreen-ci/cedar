package model

import (
	"fmt"
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
			TaskID:         "task_id",
			Execution:      2,
			TestName:       "test_name",
			Trial:          3,
			Status:         "pass",
			LineNum:        102,
			TaskCreateTime: time.Now().Add(-time.Hour),
			TestStartTime:  time.Now().Add(-30 * time.Minute),
			TestEndTime:    time.Now(),
		}
		expected := &APITestResult{
			TaskID:         utility.ToStringPtr(tr.TaskID),
			Execution:      tr.Execution,
			TestName:       utility.ToStringPtr(tr.TestName),
			Trial:          tr.Trial,
			Status:         utility.ToStringPtr(tr.Status),
			LogURL:         utility.ToStringPtr(fmt.Sprintf("https://cedar.mongodb.com/rest/v1/buildlogger/test_name/%s/%s?execution=%d", tr.TaskID, tr.TestName, tr.Execution)),
			LineNum:        tr.LineNum,
			TaskCreateTime: NewTime(tr.TaskCreateTime),
			TestStartTime:  NewTime(tr.TestStartTime),
			TestEndTime:    NewTime(tr.TestEndTime),
		}
		apiTestResult := &APITestResult{}
		assert.NoError(t, apiTestResult.Import(tr))
		assert.Equal(t, expected, apiTestResult)
	})
}
