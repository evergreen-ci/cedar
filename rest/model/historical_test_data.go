package model

import (
	"strings"

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

// StartAtKey returns the start_at key parameter that can be used to paginate
// and start at this element.
func (a *APIAggregatedHistoricalTestData) StartAtKey() string {
	return HTDStartAtKey{
		date:     a.Date.String(),
		variant:  FromAPIString(a.Variant),
		taskName: FromAPIString(a.TaskName),
		testName: FromAPIString(a.TestName),
	}.String()
}

// HTDStartAtKey is a struct used to build the start_at key parameter for
// pagination.
type HTDStartAtKey struct {
	date     string
	variant  string
	taskName string
	testName string
}

func (s HTDStartAtKey) String() string {
	elements := []string{s.date, s.variant, s.taskName, s.testName}
	return strings.Join(elements, "|")
}
