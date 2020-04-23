package model

import (
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/grip"
)

// HistoricalTestData describes aggregated test result data for a given date
// range.
type HistoricalTestData struct {
	Info            HistoricalTestDataInfo `bson:"info"`
	NumPass         int                    `bson:"num_pass"`
	NumFail         int                    `bson:"num_fail"`
	Durations       []float64              `bson:"durations"`
	AverageDuration float64                `bson:"average_duration"`
	LastUpdate      time.Time              `bson:"last_update"`

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
	Project     string    `bson:"project"`
	Variant     string    `bson:"variant"`
	TaskName    string    `bson:"task_name"`
	TestName    string    `bson:"test_name"`
	RequestType string    `bson:"request_type"`
	Date        time.Time `bson:"date"`
}

func (i *HistoricalTestDataInfo) validate() error {
	catcher := grip.NewBasicCatcher()
	catcher.NewWhen(i.Project == "", "project field must not be empty")
	catcher.NewWhen(i.Variant == "", "variant field must not be empty")
	catcher.NewWhen(i.TaskName == "", "task name field must not be empty")
	catcher.NewWhen(i.TestName == "", "test name field must not be empty")
	catcher.NewWhen(i.RequestType == "", "request type field must not be empty")
	catcher.NewWhen(i.Date.IsZero(), "date field must not be zero")

	return catcher.Resolve()
}
