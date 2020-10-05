package model

import (
	"bytes"
	"context"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/pail"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
)

const historicalTestDataCollection = "historical_test_stats"

// HistoricalTestData describes aggregated test result data for a given date
// range.
type HistoricalTestData struct {
	Info            HistoricalTestDataInfo `bson:"info"`
	NumPass         int                    `bson:"num_pass"`
	NumFail         int                    `bson:"num_fail"`
	Durations       []time.Duration        `bson:"durations"`
	AverageDuration time.Duration          `bson:"average_duration"`
	LastUpdate      time.Time              `bson:"last_update"`
	ArtifactType    PailType               `bson:"-"`

	env       cedar.Environment
	bucket    string
	populated bool
}

// CreateHistoricalTestData is an entry point for creating a new
// HistoricalTestData.
func CreateHistoricalTestData(info HistoricalTestDataInfo, artifactStorageType PailType) (*HistoricalTestData, error) {
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

// Find searches the globally configured pail bucket for the
// HistoricalTestData. The enviromemt should not be nil.
func (d *HistoricalTestData) Find(ctx context.Context) error {
	if d.env == nil {
		return errors.New("cannot find with a nil environment")
	}

	d.populated = false
	bucket, err := d.getBucket(ctx)
	if err != nil {
		return err
	}
	r, err := bucket.Get(ctx, d.getPath())
	if err != nil {
		return errors.Wrap(err, "problem getting data from bucket")
	}
	defer func() {
		grip.Error(message.WrapError(r.Close(), message.Fields{
			"message":  "problem closing bucket reader",
			"bucket":   d.bucket,
			"prefix":   "",
			"path":     d.getPath(),
			"location": d.ArtifactType,
		}))
	}()

	data, err := ioutil.ReadAll(r)
	if err != nil {
		return errors.Wrap(err, "problem reading data")
	}
	if err = bson.Unmarshal(data, d); err != nil {
		return errors.Wrap(err, "problem unmarshalling data")
	}
	d.populated = true

	return nil
}

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

	bucket, err := d.getBucket(ctx)
	if err != nil {
		return err
	}

	data, err := bson.Marshal(d)
	if err != nil {
		return errors.Wrap(err, "problem marshalling historical test data")
	}
	if err = bucket.Put(ctx, d.getPath(), bytes.NewReader(data)); err != nil {
		return errors.Wrap(err, "problem saving historical test data to bucket")
	}

	return nil
}

// Remove deletes the HistoricalTestData file from the Pail backed storage. The
// environment should not be nil.
func (d *HistoricalTestData) Remove(ctx context.Context) error {
	if d.env == nil {
		return errors.New("cannot remove with a nil environment")
	}

	bucket, err := d.getBucket(ctx)
	if err != nil {
		return err
	}

	err = bucket.Remove(ctx, d.getPath())
	if pail.IsKeyNotFoundError(err) {
		return nil
	}
	return errors.Wrap(err, "problem removing historical test data file")
}

func (d *HistoricalTestData) getBucket(ctx context.Context) (pail.Bucket, error) {
	if d.bucket == "" {
		conf := &CedarConfig{}
		conf.Setup(d.env)
		if err := conf.Find(); err != nil {
			return nil, errors.Wrap(err, "problem getting application configuration")
		}
		d.bucket = conf.Bucket.TestResultsBucket
	}

	bucket, err := d.ArtifactType.Create(
		ctx,
		d.env,
		d.bucket,
		historicalTestDataCollection,
		string(pail.S3PermissionsPrivate),
		true,
	)
	if err != nil {
		return nil, errors.Wrap(err, "problem creating bucket")
	}

	return bucket, nil
}

// HistoricalTestDataDateFormat represents the standard timestamp format for
// historical test data, which is rounded to the nearest day (YYYY-MM-DD).
const HistoricalTestDataDateFormat = "2006-01-02"

func (d *HistoricalTestData) getPath() string {
	i := d.Info
	if d.ArtifactType == PailLocal {
		return filepath.Join(i.Project, i.Variant, i.TaskName, i.TestName, i.RequestType, i.Date.Format(HistoricalTestDataDateFormat))
	}
	return filepath.Join(i.Project, i.Variant, i.TaskName, i.TestName, i.RequestType, i.Date.Format(HistoricalTestDataDateFormat))
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
