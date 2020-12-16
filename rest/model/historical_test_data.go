package model

import (
	dbmodel "github.com/evergreen-ci/cedar/model"
	"github.com/pkg/errors"
)

// APIHistoricalTestData describes aggregated test result data for a given date
// range.
type APIHistoricalTestData struct {
	Info            APIHistoricalTestDataInfo `json:"info"`
	NumPass         int                       `json:"num_pass"`
	NumFail         int                       `json:"num_fail"`
	AverageDuration float64                   `json:"average_duration"`
	LastUpdate      APITime                   `json:"last_update"`
}

// Import transforms a HistoricalTestData object into an APIHistoricalTestData
// object.
func (a *APIHistoricalTestData) Import(i interface{}) error {
	switch hd := i.(type) {
	case dbmodel.HistoricalTestData:
		a.Info = getHistoricalTestDataInfo(hd.Info)
		a.NumPass = hd.NumPass
		a.NumFail = hd.NumFail
		a.AverageDuration = hd.AverageDuration
		a.LastUpdate = NewTime(hd.LastUpdate)
	default:
		return errors.New("incorrect type when converting to APIHistoricalTestData type")
	}
	return nil
}

// APIHistoricalTestDataInfo describes information unique to a single test
// statistics document.
type APIHistoricalTestDataInfo struct {
	Project     APIString `json:"project"`
	Variant     APIString `json:"variant"`
	TaskName    APIString `json:"task_name"`
	TestName    APIString `json:"test_name"`
	RequestType APIString `json:"request_type"`
	Date        APITime   `json:"date"`
}

func getHistoricalTestDataInfo(info dbmodel.HistoricalTestDataInfo) APIHistoricalTestDataInfo {
	return APIHistoricalTestDataInfo{
		Project:     ToAPIString(info.Project),
		Variant:     ToAPIString(info.Variant),
		TaskName:    ToAPIString(info.TaskName),
		TestName:    ToAPIString(info.TestName),
		RequestType: ToAPIString(info.RequestType),
		Date:        NewTime(info.Date),
	}
}
