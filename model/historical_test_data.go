package model

import (
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/pkg/errors"
)

// HistoricalTestData describes aggregated test result data for a given date
// range.
type HistoricalTestData struct {
	Info            HistoricalTestDataInfo
	NumPass         int       `bson:"num_pass"`
	NumFail         int       `bson:"num_fail"`
	AverageDuration float64   `bson:"average_duration"`
	LastUpdate      time.Time `bson:"last_update"`

	env       cedar.Environment
	populated bool
}

// CreateHistoricalTestData is an entry point for creating a new
// HistoricalTestData.
func CreateHistoricalTestData(info HistoricalTestDataInfo) (*HistoricalTestData, error) {
	if err := info.validate(); err != nil {
		return nil, err
	}

	return &HistoricalTestData{
		Info:       info,
		LastUpdate: time.Now(),
		populated:  true,
	}, nil
}

// HistoricalTestDataInfo describes information unique to a single test
// statistics document.
type HistoricalTestDataInfo struct {
	Project   string    `bson:"project"`
	Variant   string    `bson:"variant"`
	TaskName  string    `bson:"task_name"`
	TestName  string    `bson:"test_name"`
	Requester string    `bson:"requester"`
	Date      time.Time `bson:"date"`
}

func (i *HistoricalTestDataInfo) validate() error {
	var emptyField bool
	switch {
	case i.Project == "":
		emptyField = true
	case i.Variant == "":
		emptyField = true
	case i.TaskName == "":
		emptyField = true
	case i.TestName == "":
		emptyField = true
	case i.Requester == "":
		emptyField = true
	case i.Date.IsZero():
		emptyField = true
	}

	if emptyField {
		return errors.New("all fields in HistoricalTestDataInfo must be populated")
	}
	return nil
}
