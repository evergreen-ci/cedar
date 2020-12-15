package model

import (
	"context"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

const historicalTestDataCollection = "historical_test_data"

// HistoricalTestData describes aggregated test result data for a given date
// range.
type HistoricalTestData struct {
	Info            HistoricalTestDataInfo `json:"_id" bson:"_id"`
	NumPass         int                    `json:"num_pass" bson:"num_pass"`
	NumFail         int                    `json:"num_fail" bson:"num_fail"`
	Durations       []time.Duration        `json:"durations" bson:"durations"`
	AverageDuration time.Duration          `json:"average_duration" bson:"average_duration"`
	LastUpdate      time.Time              `json:"last_update" bson:"last_update"`

	env cedar.Environment
}

// CreateHistoricalTestData is an entry point for creating a new
// HistoricalTestData.
func CreateHistoricalTestData(info HistoricalTestDataInfo) (*HistoricalTestData, error) {
	if err := info.validate(); err != nil {
		return nil, err
	}

	info.Date = time.Date(
		info.Date.Year(),
		info.Date.Month(),
		info.Date.Day(),
		0, 0, 0, 0,
		time.UTC,
	)

	return &HistoricalTestData{
		Info:         info,
		LastUpdate:   time.Now(),
		ArtifactType: artifactStorageType,
		populated:    true,
	}, nil
}

// Setup sets the environment. The environment is required for numerous
// functions on HistoricalTestData.
func (d *HistoricalTestData) Setup(e cedar.Environment) { d.env = e }

// IsNil returns if the HistoricalTestData is populated or not.
func (d *HistoricalTestData) IsNil() bool { return !d.populated }

// TODO: fix this.
// Find searches the globally configured pail bucket for the
// HistoricalTestData. The enviromemt should not be nil.
func (d *HistoricalTestData) Find(ctx context.Context) error {
	if d.env == nil {
		return errors.New("cannot find with a nil environment")
	}

	return nil
}

func (d *HistoricalTestData) Update(ctx context.Context) error {
	if !d.populated {
		return errors.New("cannot save unpopulated historical test data")
	}
	if d.env == nil {
		return errors.New("cannot find with a nil environment")
	}

}

// TODO: fix this
// Save saves HistoricalTestData to the Pail backed storage. The
// HistoricalTestData should be populated and the environment should not be
// nil.
func (d *HistoricalTestData) Save(ctx context.Context) error {
	if !d.populated {
		return errors.New("cannot save unpopulated historical test data")
	}
	if d.env == nil {
		return errors.New("cannot save with a nil environment")
	}

	d.LastUpdate = time.Now()

	return nil
}

// TODO: fix this
// Remove deletes the HistoricalTestData file from the Pail backed storage. The
// environment should not be nil.
func (d *HistoricalTestData) Remove(ctx context.Context) error {
	if d.env == nil {
		return errors.New("cannot remove with a nil environment")
	}
}

// HistoricalTestDataDateFormat represents the standard timestamp format for
// historical test data, which is rounded to the nearest day (YYYY-MM-DD).
const HistoricalTestDataDateFormat = "2006-01-02"

// HistoricalTestDataInfo describes information unique to a single test
// statistics document.
type HistoricalTestDataInfo struct {
	Project     string    `json:"project" bson:"project"`
	Variant     string    `json:"variant" bson:"variant"`
	TaskName    string    `json:"task_name" bson:"task_name"`
	TestName    string    `json:"test_name" bson:"test_name"`
	RequestType string    `json:"request_type" bson:"request_type"`
	Date        time.Time `json:"date" bson:"date"`
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
