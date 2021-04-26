package model

import (
	"strings"

	dbmodel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/utility"
	"github.com/pkg/errors"
)

// APIAggregatedHistoricalTestData describes aggregated test result data for a
// given date range.
type APIAggregatedHistoricalTestData struct {
	// In order to maintain backwards compatibility with Evergreen's
	// historical test stats, the json tags must match those in Evergreen.
	TestName *string `json:"test_file"`
	TaskName *string `json:"task_name,omitempty"`
	Variant  *string `json:"variant,omitempty"`
	Date     APITime `json:"date"`

	NumPass         int     `json:"num_pass"`
	NumFail         int     `json:"num_fail"`
	AverageDuration float64 `json:"avg_duration_pass"`
}

// Import transforms an AggregatedHistoricalTestData object into an
// APIAggregatedHistoricalTestData object.
func (a *APIAggregatedHistoricalTestData) Import(i interface{}) error {
	switch hd := i.(type) {
	case dbmodel.AggregatedHistoricalTestData:
		a.TestName = utility.ToStringPtr(hd.TestName)
		a.TaskName = utility.ToStringPtr(hd.TaskName)
		a.Variant = utility.ToStringPtr(hd.Variant)
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
		variant:  utility.FromStringPtr(a.Variant),
		taskName: utility.FromStringPtr(a.TaskName),
		testName: utility.FromStringPtr(a.TestName),
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
