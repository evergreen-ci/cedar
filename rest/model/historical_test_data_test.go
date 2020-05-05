package model

import (
	"testing"
	"time"

	dbmodel "github.com/evergreen-ci/cedar/model"
	"github.com/stretchr/testify/assert"
)

func TestHistoricalTestDataImport(t *testing.T) {
	t.Run("InvalidType", func(t *testing.T) {
		apiHistoricalTestData := &APIHistoricalTestData{}
		assert.Error(t, apiHistoricalTestData.Import(dbmodel.TestResults{}))
	})
	t.Run("ValidHistoricalTestData", func(t *testing.T) {
		tr := dbmodel.HistoricalTestData{
			Info: dbmodel.HistoricalTestDataInfo{
				Project:     "project",
				Variant:     "variant",
				TaskName:    "task_name",
				TestName:    "test_name",
				RequestType: "request_type",
				Date:        time.Now(),
			},
			NumPass:         2,
			NumFail:         2,
			Durations:       []float64{2.4, 2.65, 0.23, 1.34},
			AverageDuration: 1.655,
			LastUpdate:      time.Now(),
		}
		expected := &APIHistoricalTestData{
			Info: APIHistoricalTestDataInfo{
				Project:     ToAPIString(tr.Info.Project),
				Variant:     ToAPIString(tr.Info.Variant),
				TaskName:    ToAPIString(tr.Info.TaskName),
				TestName:    ToAPIString(tr.Info.TestName),
				RequestType: ToAPIString(tr.Info.RequestType),
				Date:        NewTime(tr.Info.Date),
			},
			NumPass:         tr.NumPass,
			NumFail:         tr.NumFail,
			Durations:       tr.Durations,
			AverageDuration: tr.AverageDuration,
			LastUpdate:      NewTime(tr.LastUpdate),
		}
		apiHistoricalTestData := &APIHistoricalTestData{}
		assert.NoError(t, apiHistoricalTestData.Import(tr))
		assert.Equal(t, expected, apiHistoricalTestData)
	})
}
