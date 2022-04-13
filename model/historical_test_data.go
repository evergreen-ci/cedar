package model

import (
	"context"
	"crypto/sha1"
	"fmt"
	"hash"
	"io"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/anser/bsonutil"
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
	ID              string                 `bson:"_id"`
	Info            HistoricalTestDataInfo `bson:"info"`
	NumPass         int                    `bson:"num_pass"`
	NumFail         int                    `bson:"num_fail"`
	AverageDuration time.Duration          `bson:"average_duration"`
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

	info.Date = utility.GetUTCDay(info.Date)

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

// Find searches the DB for the HistoricalTestData by ID. The environmemt
// should not be nil.
func (d *HistoricalTestData) Find(ctx context.Context) error {
	if d.env == nil {
		return errors.New("cannot find with a nil environment")
	}

	if d.ID == "" {
		d.ID = d.Info.ID()
	}

	d.populated = false
	if err := d.env.GetDB().Collection(historicalTestDataCollection).FindOne(ctx, bson.M{"_id": d.ID}).Decode(d); err != nil {
		return errors.Wrapf(err, "finding historical test data record '%s'", d.ID)
	}
	d.populated = true

	return nil
}

// Update updates the HistoricalTestData with the new data. If the
// HistoricalTestData does not exist, it is created. The HistoricalTestData
// should be populated and the environment should not be nil.
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
				historicalTestDataAverageDurationKey: bson.M{"$toLong": bson.M{"$cond": bson.M{
					"if": bson.M{"$gte": []interface{}{fmt.Sprintf("$%s", historicalTestDataAverageDurationKey), 1}},
					"then": bson.M{"$divide": []interface{}{
						bson.M{"$add": []interface{}{
							result.TestEndTime.Sub(result.TestStartTime),
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
					"else": result.TestEndTime.Sub(result.TestStartTime),
				}}},
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
		options.FindOneAndUpdate().SetUpsert(true),
		options.FindOneAndUpdate().SetReturnDocument(options.After),
		options.FindOneAndUpdate().SetMaxTime(time.Minute),
	).Decode(d)
	grip.DebugWhen(err == nil, message.Fields{
		"collection": historicalTestDataCollection,
		"id":         d.ID,
		"op":         "update historical test data record",
	})
	if err != nil {
		return errors.Wrapf(err, "updating historical test data '%s'", d.ID)
	}

	d.populated = true

	return nil
}

// Remove deletes the HistoricalTestData file from DB. The environment
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

	return errors.Wrapf(err, "removing historical test data record '%s'", d.ID)
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

var (
	historicalTestDataInfoProjectKey     = bsonutil.MustHaveTag(HistoricalTestDataInfo{}, "Project")
	historicalTestDataInfoVariantKey     = bsonutil.MustHaveTag(HistoricalTestDataInfo{}, "Variant")
	historicalTestDataInfoTaskNameKey    = bsonutil.MustHaveTag(HistoricalTestDataInfo{}, "TaskName")
	historicalTestDataInfoTestNameKey    = bsonutil.MustHaveTag(HistoricalTestDataInfo{}, "TestName")
	historicalTestDataInfoRequestTypeKey = bsonutil.MustHaveTag(HistoricalTestDataInfo{}, "RequestType")
	historicalTestDataInfoDateKey        = bsonutil.MustHaveTag(HistoricalTestDataInfo{}, "Date")
	historicalTestDataInfoSchemaKey      = bsonutil.MustHaveTag(HistoricalTestDataInfo{}, "Schema")
)

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

///////////////////
// Find aggregation
///////////////////

// AggregatedHistoricalTestData represents aggregated test execution data.
type AggregatedHistoricalTestData struct {
	TestName string    `bson:"test_name"`
	TaskName string    `bson:"task_name,omitempty"`
	Variant  string    `bson:"variant,omitempty"`
	Date     time.Time `bson:"date"`

	NumPass         int           `bson:"num_pass"`
	NumFail         int           `bson:"num_fail"`
	AverageDuration time.Duration `bson:"avg_duration"`
}

var (
	aggregatedHistoricalTestDataTestNameKey    = bsonutil.MustHaveTag(AggregatedHistoricalTestData{}, "TestName")
	aggregatedHistoricalTestDataTaskNameKey    = bsonutil.MustHaveTag(AggregatedHistoricalTestData{}, "TaskName")
	aggregatedHistoricalTestDataVariantKey     = bsonutil.MustHaveTag(AggregatedHistoricalTestData{}, "Variant")
	aggregatedHistoricalTestDataDateKey        = bsonutil.MustHaveTag(AggregatedHistoricalTestData{}, "Date")
	aggregatedHistoricalTestDataNumPassKey     = bsonutil.MustHaveTag(AggregatedHistoricalTestData{}, "NumPass")
	aggregatedHistoricalTestDataNumFailKey     = bsonutil.MustHaveTag(AggregatedHistoricalTestData{}, "NumFail")
	aggregatedHistoricalTestDataAvgDurationKey = bsonutil.MustHaveTag(AggregatedHistoricalTestData{}, "AverageDuration")
)

// GetHistoricalTestData queries the historical test data using a filter.
func GetHistoricalTestData(ctx context.Context, env cedar.Environment, filter HistoricalTestDataFilter) ([]AggregatedHistoricalTestData, error) {
	err := filter.Validate()
	if err != nil {
		return nil, errors.Wrap(err, "the provided HistoricalTestDataFilter is invalid")
	}

	var data []AggregatedHistoricalTestData
	pipeline := filter.queryPipeline()
	cursor, err := env.GetDB().Collection(historicalTestDataCollection).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, errors.Wrap(err, "aggregating test data")
	}
	if err = cursor.All(ctx, &data); err != nil {
		return nil, errors.Wrap(err, "unmarshalling aggregated test data")
	}

	return data, nil
}

const htdMaxQueryLimit = 1001

// HTDGroupBy represents the possible groupings of historical test data.
type HTDGroupBy string

const (
	HTDGroupByTest    HTDGroupBy = "test"
	HTDGroupByTask    HTDGroupBy = "task"
	HTDGroupByVariant HTDGroupBy = "variant"
)

func (gb HTDGroupBy) validate() error {
	switch gb {
	case HTDGroupByVariant, HTDGroupByTask, HTDGroupByTest:
	default:
		return errors.Errorf("invalid HTDGroupBy value '%s'", gb)
	}

	return nil
}

// HTDSort represents the sorting options for historical test data.
type HTDSort string

const (
	HTDSortEarliestFirst HTDSort = "earliest"
	HTDSortLatestFirst   HTDSort = "latest"
)

func (s HTDSort) validate() error {
	switch s {
	case HTDSortEarliestFirst, HTDSortLatestFirst:
	default:
		return errors.Errorf("invalid HTDSort value '%s'", s)
	}

	return nil
}

// HTDStartAt represents parameters that allow a search query to resume at a
// specific point. Used for pagination.
type HTDStartAt struct {
	Date    time.Time
	Variant string
	Task    string
	Test    string
}

func (s *HTDStartAt) validate(groupBy HTDGroupBy) error {
	if s == nil {
		return errors.New("StartAt should not be nil")
	}

	catcher := grip.NewBasicCatcher()
	catcher.NewWhen(!s.Date.Equal(utility.GetUTCDay(s.Date)), "invalid StartAt Date value")
	catcher.NewWhen(len(s.Test) == 0, "missing StartAt Test value")
	switch groupBy {
	case HTDGroupByVariant:
		catcher.NewWhen(len(s.Variant) == 0, "missing StartAt Variant value")
		fallthrough
	case HTDGroupByTask:
		catcher.NewWhen(len(s.Task) == 0, "missing StartAt Task value")
	}
	return catcher.Resolve()
}

// HistoricalTestDataFilter represents search and aggregation parameters when
// querying the historical test data.
type HistoricalTestDataFilter struct {
	Project    string
	Requesters []string
	AfterDate  time.Time
	BeforeDate time.Time

	Tests    []string
	Tasks    []string
	Variants []string

	GroupNumDays int
	GroupBy      HTDGroupBy
	StartAt      *HTDStartAt
	Limit        int
	Sort         HTDSort
}

// Validate ensures that the HistoricalTestDataFilter is valid.
func (f *HistoricalTestDataFilter) Validate() error {
	if f == nil {
		return errors.New("historical test data filter should not be nil")
	}

	catcher := grip.NewBasicCatcher()
	catcher.NewWhen(len(f.Requesters) == 0, "missing Requesters values")
	catcher.NewWhen(f.GroupNumDays <= 0, "invalid GroupNumDays value")
	catcher.NewWhen(f.Limit > htdMaxQueryLimit || f.Limit <= 0, "invalid Limit value")
	catcher.AddWhen(f.StartAt != nil, f.StartAt.validate(f.GroupBy))
	catcher.NewWhen(len(f.Tests) == 0 && len(f.Tasks) == 0, "missing Tests or Tasks values")
	catcher.Add(f.Sort.validate())
	catcher.Add(f.GroupBy.validate())
	catcher.Add(f.validateDates())
	return catcher.Resolve()
}

func (f *HistoricalTestDataFilter) validateDates() error {
	catcher := grip.NewBasicCatcher()
	catcher.NewWhen(!f.AfterDate.Equal(utility.GetUTCDay(f.AfterDate)), "invalid AfterDate value")
	catcher.NewWhen(!f.BeforeDate.Equal(utility.GetUTCDay(f.BeforeDate)), "invalid BeforeDate value")
	catcher.NewWhen(!f.BeforeDate.After(f.AfterDate), "invalid AfterDate/BeforeDate values")
	return catcher.Resolve()
}

// queryPipline creates an aggregation pipeline to query historical test data.
func (f HistoricalTestDataFilter) queryPipeline() []bson.M {
	matchExpr := f.buildMatchStage()

	return []bson.M{
		matchExpr,
		buildAddFieldsDateStage(
			historicalTestDataInfoDateKey,
			bsonutil.GetDottedKeyName(historicalTestDataInfoKey, historicalTestDataInfoDateKey),
			f.AfterDate,
			f.BeforeDate,
			f.GroupNumDays,
		),
		{"$group": bson.M{
			"_id":                                  buildHTDGroupId(f.GroupBy),
			aggregatedHistoricalTestDataNumPassKey: bson.M{"$sum": "$" + historicalTestDataNumPassKey},
			aggregatedHistoricalTestDataNumFailKey: bson.M{"$sum": "$" + historicalTestDataNumFailKey},
			"total_duration_pass": bson.M{
				"$sum": bson.M{
					"$multiply": []interface{}{
						"$" + historicalTestDataNumPassKey,
						"$" + historicalTestDataAverageDurationKey,
					},
				},
			},
		}},
		{"$project": bson.M{
			aggregatedHistoricalTestDataTestNameKey: "$_id." + aggregatedHistoricalTestDataTestNameKey,
			aggregatedHistoricalTestDataTaskNameKey: "$_id." + aggregatedHistoricalTestDataTaskNameKey,
			aggregatedHistoricalTestDataVariantKey:  "$_id." + aggregatedHistoricalTestDataVariantKey,
			aggregatedHistoricalTestDataDateKey:     "$_id." + aggregatedHistoricalTestDataDateKey,
			aggregatedHistoricalTestDataNumPassKey:  1,
			aggregatedHistoricalTestDataNumFailKey:  1,
			aggregatedHistoricalTestDataAvgDurationKey: bson.M{"$toLong": bson.M{
				"$cond": bson.M{
					"if":   bson.M{"$ne": []interface{}{"$" + aggregatedHistoricalTestDataNumPassKey, 0}},
					"then": bson.M{"$divide": []interface{}{"$total_duration_pass", "$" + aggregatedHistoricalTestDataNumPassKey}},
					"else": nil,
				},
			}},
		}},
		{"$sort": bson.D{
			{Key: aggregatedHistoricalTestDataDateKey, Value: sortDateOrder(f.Sort)},
			{Key: aggregatedHistoricalTestDataVariantKey, Value: 1},
			{Key: aggregatedHistoricalTestDataTaskNameKey, Value: 1},
			{Key: aggregatedHistoricalTestDataTestNameKey, Value: 1},
		}},
		{"$limit": f.Limit},
	}
}

// buildMatchStage builds the match stage of the query pipeline based on the
// filter options.
func (f HistoricalTestDataFilter) buildMatchStage() bson.M {
	match := bson.M{
		bsonutil.GetDottedKeyName(historicalTestDataInfoKey, historicalTestDataInfoDateKey): bson.M{
			"$gte": f.AfterDate,
			"$lt":  f.BeforeDate,
		},
		bsonutil.GetDottedKeyName(historicalTestDataInfoKey, historicalTestDataInfoProjectKey):     f.Project,
		bsonutil.GetDottedKeyName(historicalTestDataInfoKey, historicalTestDataInfoRequestTypeKey): bson.M{"$in": f.Requesters},
	}
	if len(f.Tests) > 0 {
		match[bsonutil.GetDottedKeyName(historicalTestDataInfoKey, historicalTestDataInfoTestNameKey)] = bson.M{"$in": f.Tests}
	}
	if len(f.Tasks) > 0 {
		match[bsonutil.GetDottedKeyName(historicalTestDataInfoKey, historicalTestDataInfoTaskNameKey)] = bson.M{"$in": f.Tasks}
	}
	if len(f.Variants) > 0 {
		match[bsonutil.GetDottedKeyName(historicalTestDataInfoKey, historicalTestDataInfoVariantKey)] = bson.M{"$in": f.Variants}
	}

	if f.StartAt != nil {
		match["$or"] = f.buildTestPaginationOrBranches()
	}

	return bson.M{"$match": match}
}

// buildAddFieldsDateStage builds the $addFields stage that sets the start date
// of the grouped period the stats document belongs in.
func buildAddFieldsDateStage(fieldName string, inputDateFieldName string, start time.Time, end time.Time, numDays int) bson.M {
	inputDateFieldRef := "$" + inputDateFieldName
	if numDays <= 1 {
		return bson.M{"$addFields": bson.M{fieldName: inputDateFieldRef}}
	}
	boundaries := dateBoundaries(start, end, numDays)
	branches := make([]bson.M, len(boundaries))
	for i := 0; i < len(boundaries)-1; i++ {
		branches[i] = bson.M{
			"case": bson.M{
				"$and": []interface{}{
					bson.M{"$gte": []interface{}{inputDateFieldRef, boundaries[i]}},
					bson.M{"$lt": []interface{}{inputDateFieldRef, boundaries[i+1]}},
				},
			},
			"then": boundaries[i],
		}
	}
	lastIndex := len(boundaries) - 1
	branches[lastIndex] = bson.M{
		"case": bson.M{"$gte": []interface{}{inputDateFieldRef, boundaries[lastIndex]}},
		"then": boundaries[lastIndex],
	}
	return bson.M{"$addFields": bson.M{fieldName: bson.M{"$switch": bson.M{"branches": branches}}}}
}

// buildHTDGroupId builds the _id field for the $group stage corresponding to
// the HTDGroupBy value.
func buildHTDGroupId(groupBy HTDGroupBy) bson.M {
	id := bson.M{aggregatedHistoricalTestDataDateKey: "$" + historicalTestDataInfoDateKey}
	switch groupBy {
	case HTDGroupByVariant:
		id[aggregatedHistoricalTestDataVariantKey] = "$" + bsonutil.GetDottedKeyName(historicalTestDataInfoKey, historicalTestDataInfoVariantKey)
		fallthrough
	case HTDGroupByTask:
		id[aggregatedHistoricalTestDataTaskNameKey] = "$" + bsonutil.GetDottedKeyName(historicalTestDataInfoKey, historicalTestDataInfoTaskNameKey)
		fallthrough
	case HTDGroupByTest:
		id[aggregatedHistoricalTestDataTestNameKey] = "$" + bsonutil.GetDottedKeyName(historicalTestDataInfoKey, historicalTestDataInfoTestNameKey)
	}
	return id
}

// getNextDate returns the date of the grouping period following the one
// specified in startAt.
func (f HistoricalTestDataFilter) getNextDate() time.Time {
	numDays := time.Duration(f.GroupNumDays) * 24 * time.Hour
	if f.Sort == HTDSortLatestFirst {
		return f.StartAt.Date.Add(-numDays)
	} else {
		return f.StartAt.Date.Add(numDays)
	}
}

// buildTestPaginationOrBranches builds an expression for the conditions
// imposed by the filter StartAt field.
func (f HistoricalTestDataFilter) buildTestPaginationOrBranches() []bson.M {
	var dateDescending = f.Sort == HTDSortLatestFirst
	var nextDate interface{}

	if f.GroupNumDays > 1 {
		nextDate = f.getNextDate()
	}

	fields := []htdPaginationField{
		{
			field:      bsonutil.GetDottedKeyName(historicalTestDataInfoKey, historicalTestDataInfoDateKey),
			descending: dateDescending,
			strict:     true,
			value:      f.StartAt.Date,
			nextValue:  nextDate,
		},
	}
	switch f.GroupBy {
	case HTDGroupByVariant:
		fields = append(fields, htdPaginationField{
			field:  bsonutil.GetDottedKeyName(historicalTestDataInfoKey, historicalTestDataInfoVariantKey),
			strict: true,
			value:  f.StartAt.Variant,
		})
		fallthrough
	case HTDGroupByTask:
		fields = append(fields, htdPaginationField{
			field:  bsonutil.GetDottedKeyName(historicalTestDataInfoKey, historicalTestDataInfoTaskNameKey),
			strict: true,
			value:  f.StartAt.Task,
		})
		fallthrough
	case HTDGroupByTest:
		fields = append(fields, htdPaginationField{
			field:  bsonutil.GetDottedKeyName(historicalTestDataInfoKey, historicalTestDataInfoTestNameKey),
			strict: false,
			value:  f.StartAt.Test,
		})
	}

	return buildPaginationOrBranches(fields)
}

// buildPaginationOrBranches builds and returns the $or branches of the
// pagination constraints. fields is an array of field names, and they must be
// in the same order as the sort order.
func buildPaginationOrBranches(fields []htdPaginationField) []bson.M {
	baseConstraints := bson.M{}
	branches := []bson.M{}

	for _, field := range fields {
		branch := bson.M{}
		for k, v := range baseConstraints {
			branch[k] = v
		}
		branch[field.field] = field.getNextExpression()
		branches = append(branches, branch)
		baseConstraints[field.field] = field.getEqExpression()
	}
	return branches
}

// dateBoundaries returns the date boundaries when splitting the period between
// 'start' and 'end' in groups of 'numDays' days. The boundaries are the start
// dates of the periods of 'numDays' (or less for the last period), starting
// with 'start'.
func dateBoundaries(start time.Time, end time.Time, numDays int) []time.Time {
	if numDays <= 0 {
		numDays = 1
	}

	start = utility.GetUTCDay(start)
	end = utility.GetUTCDay(end)
	duration := 24 * time.Hour * time.Duration(numDays)
	boundary := start
	boundaries := []time.Time{}

	for boundary.Before(end) {
		boundaries = append(boundaries, boundary)
		boundary = boundary.Add(duration)
	}
	return boundaries
}

// sortDateOrder returns the sort order specification (1, -1) for the date
// field corresponding to the HTDSort value.
func sortDateOrder(sort HTDSort) int {
	if sort == HTDSortLatestFirst {
		return -1
	} else {
		return 1
	}
}

// htdPaginationField represents a historical test data document field that is
// used to determine where to resume during pagination.
type htdPaginationField struct {
	field      string
	descending bool
	strict     bool
	value      interface{}
	nextValue  interface{}
}

// getEqExpression returns an expression that can be used to match the
// documents which have the same field value or are in the same range as this
// htdPaginationField.
func (pf htdPaginationField) getEqExpression() interface{} {
	if pf.nextValue == nil {
		return pf.value
	}
	if pf.descending {
		return bson.M{
			"$lte": pf.value,
			"$gt":  pf.nextValue,
		}
	} else {
		return bson.M{
			"$gte": pf.value,
			"$lt":  pf.nextValue,
		}
	}
}

// getNextExpression returns an expression that can be used to match the
// documents which have a field value greater or smaller than this
// htdPaginationField.
func (pf htdPaginationField) getNextExpression() bson.M {
	var operator string
	var value interface{}
	var strict bool

	if pf.nextValue != nil {
		value = pf.nextValue
		strict = false
	} else {
		value = pf.value
		strict = pf.strict
	}

	if pf.descending {
		if strict {
			operator = "$lt"
		} else {
			operator = "$lte"
		}
	} else {
		if strict {
			operator = "$gt"
		} else {
			operator = "$gte"
		}
	}
	return bson.M{operator: value}
}
