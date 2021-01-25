package model

import (
	dbmodel "github.com/evergreen-ci/cedar/model"
	"github.com/pkg/errors"
)

// APIAggregatedHistoricalTestData describes aggregated test result data for a
// given date range.
type APIAggregatedHistoricalTestData struct {
	TestName APIString `json:"test_name"`
	TaskName APIString `json:"task_name,omitempty"`
	Variant  APIString `json:"variant,omitempty"`
	Date     APITime   `json:"date"`

	NumPass         int     `json:"num_pass"`
	NumFail         int     `json:"num_fail"`
	AverageDuration float64 `json:"average_duration"`
}

// Import transforms an AggregatedHistoricalTestData object into an
// APIAggregatedHistoricalTestData object.
func (a *APIAggregatedHistoricalTestData) Import(i interface{}) error {
	switch hd := i.(type) {
	case dbmodel.AggregatedHistoricalTestData:
		a.TestName = ToAPIString(hd.TestName)
		a.TaskName = ToAPIString(hd.TaskName)
		a.Variant = ToAPIString(hd.Variant)
		a.Date = NewTime(hd.Date)
		a.NumPass = hd.NumPass
		a.NumFail = hd.NumFail
		a.AverageDuration = hd.AverageDuration.Seconds()
	default:
		return errors.New("incorrect type when converting to APIHistoricalTestData type")
	}
	return nil
}
