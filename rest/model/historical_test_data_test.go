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
			Durations:       []time.Duration{2, 2, 5, 3},
			AverageDuration: 3,
			LastUpdate:      time.Now(),
		}
		var durSecs []float64
		for _, dur := range tr.Durations {
			durSecs = append(durSecs, dur.Seconds())
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
			Durations:       durSecs,
			AverageDuration: tr.AverageDuration.Seconds(),
			LastUpdate:      NewTime(tr.LastUpdate),
		}
		apiHistoricalTestData := &APIHistoricalTestData{}
		assert.NoError(t, apiHistoricalTestData.Import(tr))
		assert.Equal(t, expected, apiHistoricalTestData)
	})
}
