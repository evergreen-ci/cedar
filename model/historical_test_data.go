package model

import (
	"context"
	"crypto/sha1"
	"fmt"
	"hash"
	"io"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const historicalTestDataCollection = "historical_test_data"

// HistoricalTestData describes aggregated test result data for a given date
// range.
type HistoricalTestData struct {
	ID              string                 `bson:"_id,omitempty"`
	Info            HistoricalTestDataInfo `bson:"info"`
	NumPass         int                    `bson:"num_pass"`
	NumFail         int                    `bson:"num_fail"`
	AverageDuration float64                `bson:"average_duration"`
	LastUpdate      time.Time              `bson:"last_update"`

	env       cedar.Environment
	populated bool
}

var (
	historicalTestDataIDKey              = bsonutil.MustHaveTag(HistoricalTestData{}, "ID")
	historicalTestDataInfoKey            = bsonutil.MustHaveTag(HistoricalTestData{}, "Info")
	historicalTestDataNumPassKey         = bsonutil.MustHaveTag(HistoricalTestData{}, "NumPass")
	historicalTestDataNumFailKey         = bsonutil.MustHaveTag(HistoricalTestData{}, "NumFail")
	historicalTestDataAverageDurationKey = bsonutil.MustHaveTag(HistoricalTestData{}, "AverageDuration")
	historicalTestDataLastUpdateKey      = bsonutil.MustHaveTag(HistoricalTestData{}, "LastUpdate")
)

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
		ID:        info.ID(),
		Info:      info,
		populated: true,
	}, nil
}

// Setup sets the environment. The environment is required for numerous
// functions on HistoricalTestData.
func (d *HistoricalTestData) Setup(e cedar.Environment) { d.env = e }

// IsNil returns if the HistoricalTestData is populated or not.
func (d *HistoricalTestData) IsNil() bool { return !d.populated }

// Find searches the database for the HistoricalTestData. The enviromemt should
// not be nil.
func (d *HistoricalTestData) Find(ctx context.Context) error {
	if d.env == nil {
		return errors.New("cannot find with a nil environment")
	}

	if d.ID == "" {
		d.ID = d.Info.ID()
	}

	d.populated = false
	err := d.env.GetDB().Collection(historicalTestDataCollection).FindOne(ctx, bson.M{"_id": d.ID}).Decode(d)
	if db.ResultsNotFound(err) {
		return errors.Wrapf(err, "could not find historical test data record with id %s in the database", d.ID)
	} else if err != nil {
		return errors.Wrap(err, "problem finding historical test data record")
	}
	d.populated = true

	return nil
}

// Update updates the HistoricalTestData with the new data. If the
// HistoricalTestData, it is created. The HistoricalTestData should be
// populated and the environment should not be nil.
func (d *HistoricalTestData) Update(ctx context.Context, result TestResult) error {
	if !d.populated {
		return errors.New("cannot update unpopulated historical test data")
	}
	if d.env == nil {
		return errors.New("cannot update with a nil environment")
	}

	if d.ID == "" {
		d.ID = d.Info.ID()
	}

	d.populated = false

	query := bson.M{
		historicalTestDataIDKey:   d.ID,
		historicalTestDataInfoKey: d.Info,
	}
	pipeline := []bson.M{
		{"$set": bson.M{historicalTestDataLastUpdateKey: "$$NOW"}},
	}
	switch result.Status {
	case "pass":
		pipeline = append(
			pipeline,
			bson.M{"$set": bson.M{
				historicalTestDataAverageDurationKey: bson.M{"$cond": bson.M{
					"if": bson.M{"$gte": []interface{}{fmt.Sprintf("$%s", historicalTestDataAverageDurationKey), 1}},
					"then": bson.M{"$divide": []interface{}{
						bson.M{"$add": []interface{}{
							result.TestEndTime.Sub(result.TestStartTime).Seconds(),
							bson.M{"$multiply": []interface{}{
								fmt.Sprintf("$%s", historicalTestDataNumPassKey),
								fmt.Sprintf("$%s", historicalTestDataAverageDurationKey),
							}},
						}},
						bson.M{"$add": []interface{}{
							fmt.Sprintf("$%s", historicalTestDataNumPassKey),
							1,
						}},
					}},
					"else": result.TestEndTime.Sub(result.TestStartTime).Seconds(),
				}},
			}},
			bson.M{"$set": bson.M{
				historicalTestDataNumPassKey: bson.M{"$cond": bson.M{
					"if": bson.M{"$gte": []interface{}{fmt.Sprintf("$%s", historicalTestDataNumPassKey), 0}},
					"then": bson.M{"$add": []interface{}{
						fmt.Sprintf("$%s", historicalTestDataNumPassKey),
						1,
					}},
					"else": 1,
				}},
			}},
		)
	case "fail", "silentfail":
		pipeline = append(
			pipeline,
			bson.M{"$set": bson.M{
				historicalTestDataNumFailKey: bson.M{"$cond": bson.M{
					"if": bson.M{"$gte": []interface{}{fmt.Sprintf("$%s", historicalTestDataNumFailKey), 0}},
					"then": bson.M{"$add": []interface{}{
						fmt.Sprintf("$%s", historicalTestDataNumFailKey),
						1,
					}},
					"else": 1,
				}},
			}},
		)
	}

	err := d.env.GetDB().Collection(historicalTestDataCollection).FindOneAndUpdate(
		ctx,
		query,
		pipeline,
		options.FindOneAndUpdate().SetReturnDocument(options.After),
		options.FindOneAndUpdate().SetUpsert(true),
	).Decode(d)
	grip.DebugWhen(err == nil, message.Fields{
		"collection": historicalTestDataCollection,
		"id":         d.ID,
		"op":         "update historical test data record",
	})
	if err != nil {
		return errors.Wrapf(err, "problem updating historical test data with id %s", d.ID)
	}

	d.populated = true

	return nil
}

// Remove deletes the HistoricalTestData file from database. The environment
// should not be nil.
func (d *HistoricalTestData) Remove(ctx context.Context) error {
	if d.env == nil {
		return errors.New("cannot remove with a nil environment")
	}

	if d.ID == "" {
		d.ID = d.Info.ID()
	}

	deleteResult, err := d.env.GetDB().Collection(historicalTestDataCollection).DeleteOne(ctx, bson.M{"_id": d.ID})
	grip.DebugWhen(err == nil, message.Fields{
		"collection":   historicalTestDataCollection,
		"id":           d.ID,
		"deleteResult": deleteResult,
		"op":           "remove historical test data record",
	})

	return errors.Wrapf(err, "problem removing test results record with id %s", d.ID)
}

// HistoricalTestDataDateFormat represents the standard timestamp format for
// historical test data, which is rounded to the nearest day (YYYY-MM-DD).
const HistoricalTestDataDateFormat = "2006-01-02"

// HistoricalTestDataInfo describes information unique to a single test
// statistics document.
type HistoricalTestDataInfo struct {
	Project     string    `bson:"project"`
	Variant     string    `bson:"variant"`
	TaskName    string    `bson:"task_name"`
	TestName    string    `bson:"test_name"`
	RequestType string    `bson:"request_type"`
	Date        time.Time `bson:"date"`
	Schema      int       `bson:"schema,omitempty"`
}

// ID creates a unique hash for a HistoricalTestData record.
func (id *HistoricalTestDataInfo) ID() string {
	var hash hash.Hash

	if id.Schema == 0 {
		hash = sha1.New()
		_, _ = io.WriteString(hash, id.Project)
		_, _ = io.WriteString(hash, id.Variant)
		_, _ = io.WriteString(hash, id.TaskName)
		_, _ = io.WriteString(hash, id.TestName)
		_, _ = io.WriteString(hash, id.RequestType)
		_, _ = io.WriteString(hash, id.Date.Format(HistoricalTestDataDateFormat))
	} else {
		panic("unsupported schema")
	}

	return fmt.Sprintf("%x", hash.Sum(nil))
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
