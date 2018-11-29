package model

import (
	"crypto/sha1"
	"fmt"
	"hash"
	"io"
	"sort"
	"time"

	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/util"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/anser/model"
	"github.com/mongodb/ftdc/events"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"gopkg.in/mgo.v2/bson"
)

const perfResultCollection = "perf_results"

type PerformanceResult struct {
	ID          string                `bson:"_id,omitempty"`
	Info        PerformanceResultInfo `bson:"info,omitempty"`
	CreatedAt   time.Time             `bson:"created_ts"`
	CompletedAt time.Time             `bson:"completed_at"`
	Version     int                   `bson:"version,omitempty"`

	// The source timeseries data is stored in a remote location,
	// we'll probably need to store an identifier so we know which
	// service to use to access that data. We'd then summarize
	// that data and store it in the document.
	//
	// The structure has a both a schema to describe the layout
	// the data (e.g. raw, results,) format (e.g. bson/ftdc/json),
	// and tags to describe the source (e.g. user submitted,
	// generated.)
	Artifacts []ArtifactInfo `bson:"artifacs,omitempty"`

	// Total represents the sum of all events, and used for tests
	// that report a single summarized event rather than a
	// sequence of timeseries points. Is omitted except when
	// provided by the test.
	Total *events.Performance `bson:"total,omitempty"`

	Rollups *PerfRollups `bson:"rollups,omitempty"`

	env       sink.Environment
	populated bool
}

var (
	perfIDKey        = bsonutil.MustHaveTag(PerformanceResult{}, "ID")
	perfInfoKey      = bsonutil.MustHaveTag(PerformanceResult{}, "Info")
	perfArtifactsKey = bsonutil.MustHaveTag(PerformanceResult{}, "Artifacts")
	perfRollupsKey   = bsonutil.MustHaveTag(PerformanceResult{}, "Rollups")
	perfTotalKey     = bsonutil.MustHaveTag(PerformanceResult{}, "Total")
	perfVersionlKey  = bsonutil.MustHaveTag(PerformanceResult{}, "Version")
)

func CreatePerformanceResult(info PerformanceResultInfo, source []ArtifactInfo) *PerformanceResult {
	createdAt := time.Now()

	for idx := range source {
		source[idx].CreatedAt = createdAt
	}

	return &PerformanceResult{
		ID:        info.ID(),
		Artifacts: source,
		Info:      info,
		populated: true,
	}
}

func (result *PerformanceResult) Setup(e sink.Environment) { result.env = e }
func (result *PerformanceResult) IsNil() bool              { return !result.populated }
func (result *PerformanceResult) Find() error {
	conf, session, err := sink.GetSessionWithConfig(result.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	result.populated = false
	err = session.DB(conf.DatabaseName).C(perfResultCollection).FindId(result.ID).One(result)
	if db.ResultsNotFound(err) {
		return errors.New("could not find result record in the database")
	} else if err != nil {
		return errors.Wrap(err, "problem finding result config")
	}

	result.populated = true
	if result.Rollups != nil {
		result.Rollups.id = result.ID
		result.Rollups.Setup(result.env)
	}

	return nil
}

func (result *PerformanceResult) Save() error {
	if !result.populated {
		return errors.New("cannot save non-populated result data")
	}

	if result.ID == "" {
		result.ID = result.Info.ID()
		if result.ID == "" {
			return errors.New("cannot ")
		}
	}

	conf, session, err := sink.GetSessionWithConfig(result.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	changeInfo, err := session.DB(conf.DatabaseName).C(perfResultCollection).UpsertId(result.ID, result)
	grip.DebugWhen(err == nil, message.Fields{
		"ns":     model.Namespace{DB: conf.DatabaseName, Collection: perfResultCollection},
		"id":     result.ID,
		"change": changeInfo,
		"op":     "save perf result",
	})
	return errors.Wrap(err, "problem saving perf result to collection")
}

////////////////////////////////////////////////////////////////////////
//
// Component Types

type PerformanceResultInfo struct {
	Project   string           `bson:"project,omitempty"`
	Version   string           `bson:"version,omitempty"`
	Variant   string           `bson:"variant,omitempty"`
	TaskName  string           `bson:"task_name,omitempty"`
	TaskID    string           `bson:"task_id,omitempty"`
	Execution int              `bson:"execution,omitempty"`
	TestName  string           `bson:"test_name,omitempty"`
	Trial     int              `bson:"trial,omitempty"`
	Parent    string           `bson:"parent,omitempty"`
	Tags      []string         `bson:"tags,omitempty"`
	Arguments map[string]int32 `bson:"args,omitempty"`
	Schema    int              `bson:"schema,omitempty"`
}

var (
	perfResultInfoProjectKey   = bsonutil.MustHaveTag(PerformanceResultInfo{}, "Project")
	perfResultInfoVersionKey   = bsonutil.MustHaveTag(PerformanceResultInfo{}, "Version")
	perfResultInfoVariantKey   = bsonutil.MustHaveTag(PerformanceResultInfo{}, "Variant")
	perfResultInfoTaskNameKey  = bsonutil.MustHaveTag(PerformanceResultInfo{}, "TaskName")
	perfResultInfoTaskIDKey    = bsonutil.MustHaveTag(PerformanceResultInfo{}, "TaskID")
	perfResultInfoExecutionKey = bsonutil.MustHaveTag(PerformanceResultInfo{}, "Execution")
	perfResultInfoTestNameKey  = bsonutil.MustHaveTag(PerformanceResultInfo{}, "TestName")
	perfResultInfoTrialKey     = bsonutil.MustHaveTag(PerformanceResultInfo{}, "Trial")
	perfResultInfoParentKey    = bsonutil.MustHaveTag(PerformanceResultInfo{}, "Parent")
	perfResultInfoTagsKey      = bsonutil.MustHaveTag(PerformanceResultInfo{}, "Tags")
	perfResultInfoArgumentsKey = bsonutil.MustHaveTag(PerformanceResultInfo{}, "Arguments")
)

func (id *PerformanceResultInfo) ID() string {
	var hash hash.Hash

	if id.Schema == 0 {
		hash = sha1.New()
		_, _ = io.WriteString(hash, id.Project)
		_, _ = io.WriteString(hash, id.Version)
		_, _ = io.WriteString(hash, id.Variant)
		_, _ = io.WriteString(hash, id.TaskName)
		_, _ = io.WriteString(hash, id.TaskID)
		_, _ = io.WriteString(hash, fmt.Sprint(id.Execution))
		_, _ = io.WriteString(hash, id.TestName)
		_, _ = io.WriteString(hash, fmt.Sprint(id.Trial))
		_, _ = io.WriteString(hash, id.Parent)

		sort.Strings(id.Tags)
		for _, str := range id.Tags {
			_, _ = io.WriteString(hash, str)
		}

		if len(id.Arguments) > 0 {
			args := []string{}
			for k, v := range id.Arguments {
				args = append(args, fmt.Sprintf("%s=%d", k, v))
			}

			sort.Strings(args)
			for _, str := range args {
				_, _ = io.WriteString(hash, str)
			}
		}
	} else {
		panic("unsupported schema")
	}

	return fmt.Sprintf("%x", hash.Sum(nil))
}

type PerformanceResults struct {
	Results   []PerformanceResult `bson:"results"`
	env       sink.Environment
	populated bool
}

type PerfFindOptions struct {
	Interval    util.TimeRange
	Info        PerformanceResultInfo
	MaxDepth    int
	GraphLookup bool
}

func (r *PerformanceResults) Setup(e sink.Environment) { r.env = e }
func (r *PerformanceResults) IsNil() bool              { return r.Results == nil }

// Returns the PerformanceResults that are started/completed within the given range (if completed).
func (r *PerformanceResults) Find(options PerfFindOptions) error {
	conf, session, err := sink.GetSessionWithConfig(r.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	search := make(map[string]interface{})
	if options.Info.Parent != "" { // this is the root node
		search["_id"] = options.Info.Parent
	} else {
		if options.Interval.IsZero() || !options.Interval.IsValid() {
			return errors.New("invalid time range given")
		}
		search = r.createFindQuery(options)
	}

	r.populated = false
	err = session.DB(conf.DatabaseName).C(perfResultCollection).Find(search).All(&r.Results)
	if options.Info.Parent != "" && len(r.Results) > 0 && options.MaxDepth > -1 { // i.e. the parent fits the search criteria
		if options.GraphLookup {
			err = r.findAllChildrenGraphLookup(options.Info.Parent, options.MaxDepth, options.Info.Tags)
		} else {
			err = r.findAllChildren(options.Info.Parent, options.MaxDepth)
		}
	}
	if err != nil && !db.ResultsNotFound(err) {
		return errors.WithStack(err)
	}
	r.populated = true
	return nil
}

func (r *PerformanceResults) createFindQuery(options PerfFindOptions) map[string]interface{} {
	search := bson.M{
		"created_ts":   bson.M{"$gte": options.Interval.StartAt},
		"completed_at": bson.M{"$lte": options.Interval.EndAt},
	}
	if options.Info.Project != "" {
		search[bsonutil.GetDottedKeyName("info", "project")] = options.Info.Project
	}
	if options.Info.Version != "" {
		search[bsonutil.GetDottedKeyName("info", "version")] = options.Info.Version
	}
	if options.Info.TaskName != "" {
		search[bsonutil.GetDottedKeyName("info", "task_name")] = options.Info.TaskName
	}
	if options.Info.TaskID != "" {
		search[bsonutil.GetDottedKeyName("info", "task_id")] = options.Info.TaskID
	}
	if options.Info.TestName != "" {
		search[bsonutil.GetDottedKeyName("info", "test_name")] = options.Info.TestName
	}
	if options.Info.Execution != 0 {
		search[bsonutil.GetDottedKeyName("info", "execution")] = options.Info.Execution
	}
	if options.Info.Trial != 0 {
		search[bsonutil.GetDottedKeyName("info", "trial")] = options.Info.Trial
	}
	if options.Info.Schema != 0 {
		search[bsonutil.GetDottedKeyName("info", "schema")] = options.Info.Schema
	}
	if len(options.Info.Tags) > 0 {
		search[bsonutil.GetDottedKeyName("info", "tags")] = bson.M{"$in": options.Info.Tags}
	}
	if len(options.Info.Arguments) > 0 {
		var args []bson.M
		for key, val := range options.Info.Arguments {
			args = append(args, bson.M{key: val})
		}
		search[bsonutil.GetDottedKeyName("info", "args")] = bson.M{"$in": args}
	}
	return search
}

// All children of parent are recursively added to r.Results
func (r *PerformanceResults) findAllChildren(parent string, depth int) error {
	if depth < 0 {
		return nil
	}

	search := bson.M{bsonutil.GetDottedKeyName("info", "parent"): parent}
	conf, session, err := sink.GetSessionWithConfig(r.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()
	temp := []PerformanceResult{}
	err = session.DB(conf.DatabaseName).C(perfResultCollection).Find(search).All(&temp)
	r.Results = append(r.Results, temp...)
	for _, result := range temp {
		// look into that parent
		err = r.findAllChildren(result.ID, depth-1)
	}
	return err
}

// All children of parent are recursively added to r.Results using $graphLookup
func (r *PerformanceResults) findAllChildrenGraphLookup(parent string, maxDepth int, tags []string) error {
	conf, session, err := sink.GetSessionWithConfig(r.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	match := bson.M{"$match": bson.M{"_id": parent}}
	graphLookup := bson.M{
		"$graphLookup": bson.M{
			"from":             perfResultCollection,
			"startWith":        "$" + "_id",
			"connectFromField": "_id",
			"connectToField":   bsonutil.GetDottedKeyName("info", "parent"),
			"maxDepth":         maxDepth,
			"as":               "children",
		},
	}
	var project bson.M
	if len(tags) > 0 {
		project = bson.M{
			"$project": bson.M{
				"_id": 0,
				"children": bson.M{
					"$filter": bson.M{
						"input": "$" + "children",
						"as":    "child",
						"cond": bson.M{
							"$eq": []interface{}{
								tags,
								"$$" + bsonutil.GetDottedKeyName("child", "info", "tags"),
							},
						},
					},
				},
			},
		}
	} else {
		project = bson.M{"$project": bson.M{"_id": 0, "children": 1}}
	}
	pipeline := []bson.M{
		match,
		graphLookup,
		project,
	}
	pipe := session.DB(conf.DatabaseName).C(perfResultCollection).Pipe(pipeline)
	iter := pipe.Iter()
	defer iter.Close()

	doc := struct {
		Children []PerformanceResult `bson:"children,omitempty"`
	}{}
	for iter.Next(&doc) {
		r.Results = append(r.Results, doc.Children...)
	}
	if err = iter.Err(); err != nil {
		return errors.Wrap(err, "problem getting children")
	}
	return nil
}
