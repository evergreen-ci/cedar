package model

import (
	"testing"
	"time"

	dbmodel "github.com/evergreen-ci/cedar/model"
	"github.com/stretchr/testify/assert"
)

func TestHistoricalTestDataImport(t *testing.T) {
	t.Run("InvalidType", func(t *testing.T) {
		api := &APIAggregatedHistoricalTestData{}
		assert.Error(t, api.Import(dbmodel.TestResults{}))
	})
	t.Run("ValidHistoricalTestData", func(t *testing.T) {
		tr := dbmodel.AggregatedHistoricalTestData{
			TestName:        "test_name",
			TaskName:        "task_name",
			Variant:         "variant",
			Date:            time.Now(),
			NumPass:         2,
			NumFail:         2,
			AverageDuration: 30 * time.Second,
			LastUpdate:      time.Now(),
		}
		expected := &APIAggregatedHistoricalTestData{
			TestName:        ToAPIString(tr.TestName),
			TaskName:        ToAPIString(tr.TaskName),
			Variant:         ToAPIString(tr.Variant),
			Date:            NewTime(tr.Date),
			NumPass:         tr.NumPass,
			NumFail:         tr.NumFail,
			AverageDuration: tr.AverageDuration.Seconds(),
			LastUpdate:      NewTime(tr.LastUpdate),
		}
		api := &APIAggregatedHistoricalTestData{}
		assert.NoError(t, api.Import(tr))
		assert.Equal(t, expected, api)
	})
}
