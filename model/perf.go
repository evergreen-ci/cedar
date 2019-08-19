package model

import (
	"crypto/sha1"
	"fmt"
	"hash"
	"io"
	"sort"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/util"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
)

const perfResultCollection = "perf_results"

// PerformanceResult describes a single result of a performance test from
// Evergreen.
type PerformanceResult struct {
	ID          string                `bson:"_id,omitempty"`
	Info        PerformanceResultInfo `bson:"info,omitempty"`
	CreatedAt   time.Time             `bson:"created_at"`
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
	Artifacts []ArtifactInfo `bson:"artifacts"`

	Rollups PerfRollups `bson:"rollups"`

	env       cedar.Environment
	populated bool
}

var (
	perfIDKey          = bsonutil.MustHaveTag(PerformanceResult{}, "ID")
	perfInfoKey        = bsonutil.MustHaveTag(PerformanceResult{}, "Info")
	perfCreatedAtKey   = bsonutil.MustHaveTag(PerformanceResult{}, "CreatedAt")
	perfCompletedAtKey = bsonutil.MustHaveTag(PerformanceResult{}, "CompletedAt")
	perfArtifactsKey   = bsonutil.MustHaveTag(PerformanceResult{}, "Artifacts")
	perfRollupsKey     = bsonutil.MustHaveTag(PerformanceResult{}, "Rollups")
	perfVersionlKey    = bsonutil.MustHaveTag(PerformanceResult{}, "Version")
)

// CreatePerformanceResult is the entry point for creating a performance
// result.
func CreatePerformanceResult(info PerformanceResultInfo, source []ArtifactInfo, rollups []PerfRollupValue) *PerformanceResult {
	createdAt := time.Now()

	for idx := range source {
		source[idx].CreatedAt = createdAt
	}

	return &PerformanceResult{
		ID:        info.ID(),
		Info:      info,
		Artifacts: source,
		Rollups: PerfRollups{
			id:    info.ID(),
			Stats: append([]PerfRollupValue{}, rollups...),
		},
		populated: true,
	}
}

// Setup sets the environment for the performance result. The environment is
// required for numerous functions on PerformanceResult.
func (result *PerformanceResult) Setup(e cedar.Environment) { result.env = e }

// IsNil returns if the performance result is populated or not.
func (result *PerformanceResult) IsNil() bool { return !result.populated }

// Find searches the database for the performance result. The enviromemt should
// not be nil.
func (result *PerformanceResult) Find() error {
	conf, session, err := cedar.GetSessionWithConfig(result.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	if result.ID == "" {
		result.ID = result.Info.ID()
	}

	result.populated = false
	err = session.DB(conf.DatabaseName).C(perfResultCollection).FindId(result.ID).One(result)
	if db.ResultsNotFound(err) {
		return errors.New("could not find result record in the database")
	} else if err != nil {
		return errors.Wrap(err, "problem finding result")
	}

	result.populated = true
	result.Rollups.id = result.ID

	return nil
}

// SaveNew saves a new performance result to the database, if a result with the
// same ID already exists an error is returned. The result should be populated
// and the environment should not be nil.
func (result *PerformanceResult) SaveNew() error {
	if !result.populated {
		return errors.New("cannot save unpopulated performance result")
	}
	if result.env == nil {
		return errors.New("cannot save with a nil environment")
	}
	ctx, cancel := result.env.Context()
	defer cancel()

	if result.ID == "" {
		result.ID = result.Info.ID()
	}

	insertResult, err := result.env.GetDB().Collection(perfResultCollection).InsertOne(ctx, result)
	grip.DebugWhen(err == nil, message.Fields{
		"collection":   perfResultCollection,
		"id":           result.ID,
		"insertResult": insertResult,
		"op":           "save new performance result",
	})

	return errors.Wrapf(err, "problem saving new performance result %s", result.ID)
}

// AppendArtifacts appends new artifacts to an existing performance result. The
// environment should not be nil.
func (result *PerformanceResult) AppendArtifacts(artifacts []ArtifactInfo) error {
	if result.env == nil {
		return errors.New("cannot not append artifacts with a nil environment")
	}
	if result.ID == "" {
		result.ID = result.Info.ID()
	}
	if len(artifacts) == 0 {
		grip.Warning(message.Fields{
			"collection": perfResultCollection,
			"id":         result.ID,
			"message":    "append artifacts called with no artifacts",
		})
		return nil
	}
	ctx, cancel := result.env.Context()
	defer cancel()

	updateResult, err := result.env.GetDB().Collection(perfResultCollection).UpdateOne(
		ctx,
		bson.M{"_id": result.ID},
		bson.M{
			"$push": bson.M{
				perfArtifactsKey: bson.M{"$each": artifacts},
			},
		},
	)
	grip.DebugWhen(err == nil, message.Fields{
		"collection":   perfResultCollection,
		"id":           result.ID,
		"updateResult": updateResult,
		"artifacts":    artifacts,
		"op":           "append artifacts to a perf result",
	})
	if err == nil && updateResult.MatchedCount == 0 {
		err = errors.Errorf("could not find perf result record with id %s in the database", result.ID)
	}

	return errors.Wrapf(err, "problem appending artifacts to perf result with id %s", result.ID)
}

// Remove removes the performance result from the database. The environment
// should not be nil.
func (result *PerformanceResult) Remove() (int, error) {
	if result.ID == "" {
		result.ID = result.Info.ID()
	}

	conf, session, err := cedar.GetSessionWithConfig(result.env)
	if err != nil {
		return -1, errors.WithStack(err)
	}
	defer session.Close()

	children := PerformanceResults{env: result.env}
	if err = children.findAllChildrenGraphLookup(result.ID, -1, []string{}); err != nil {
		return -1, errors.Wrap(err, "problem getting children to remove")
	}

	ids := []string{result.ID}
	for _, res := range children.Results {
		ids = append(ids, res.ID)
	}
	query := bson.M{
		"_id": bson.M{
			"$in": ids,
		},
	}
	changeInfo, err := session.DB(conf.DatabaseName).C(perfResultCollection).RemoveAll(query)
	if err != nil {
		return -1, errors.Wrap(err, "problem removing perf results")
	}
	return changeInfo.Removed, nil
}

// Close "closes out" the performance result by populating the completed_at
// field. The envirnment should not be nil.
func (result *PerformanceResult) Close(completedAt time.Time) error {
	if result.env == nil {
		return errors.New("cannot close perf result with a nil environment")
	}
	ctx, cancel := result.env.Context()
	defer cancel()

	if result.ID == "" {
		result.ID = result.Info.ID()
	}

	updateResult, err := result.env.GetDB().Collection(perfResultCollection).UpdateOne(
		ctx,
		bson.M{"_id": result.ID},
		bson.M{"$set": bson.M{perfCompletedAtKey: completedAt}},
	)
	grip.DebugWhen(err == nil, message.Fields{
		"collection":   perfResultCollection,
		"id":           result.ID,
		"completed_at": completedAt,
		"updateResult": updateResult,
		"op":           "close perf result",
	})
	if err == nil && updateResult.MatchedCount == 0 {
		err = errors.Errorf("could not find perf result record with id %s in the database", result.ID)
	}

	return errors.Wrapf(err, "problem closing perf result with id %s", result.ID)

}

////////////////////////////////////////////////////////////////////////
//
// Component Types

// PerformanceResultInfo describes information unique to a single performance
// result.
type PerformanceResultInfo struct {
	Project   string           `bson:"project,omitempty"`
	Version   string           `bson:"version,omitempty"`
	Order     int              `bson:"order,omitempty"`
	Variant   string           `bson:"variant,omitempty"`
	TaskName  string           `bson:"task_name,omitempty"`
	TaskID    string           `bson:"task_id,omitempty"`
	Execution int              `bson:"execution"`
	TestName  string           `bson:"test_name,omitempty"`
	Trial     int              `bson:"trial"`
	Parent    string           `bson:"parent,omitempty"`
	Tags      []string         `bson:"tags,omitempty"`
	Arguments map[string]int32 `bson:"args,omitempty"`
	Mainline  bool             `bson:"mainline"`
	Schema    int              `bson:"schema,omitempty"`
}

var (
	perfResultInfoProjectKey   = bsonutil.MustHaveTag(PerformanceResultInfo{}, "Project")
	perfResultInfoVersionKey   = bsonutil.MustHaveTag(PerformanceResultInfo{}, "Version")
	perfResultInfoOrderKey     = bsonutil.MustHaveTag(PerformanceResultInfo{}, "Order")
	perfResultInfoVariantKey   = bsonutil.MustHaveTag(PerformanceResultInfo{}, "Variant")
	perfResultInfoTaskNameKey  = bsonutil.MustHaveTag(PerformanceResultInfo{}, "TaskName")
	perfResultInfoTaskIDKey    = bsonutil.MustHaveTag(PerformanceResultInfo{}, "TaskID")
	perfResultInfoExecutionKey = bsonutil.MustHaveTag(PerformanceResultInfo{}, "Execution")
	perfResultInfoTestNameKey  = bsonutil.MustHaveTag(PerformanceResultInfo{}, "TestName")
	perfResultInfoTrialKey     = bsonutil.MustHaveTag(PerformanceResultInfo{}, "Trial")
	perfResultInfoParentKey    = bsonutil.MustHaveTag(PerformanceResultInfo{}, "Parent")
	perfResultInfoTagsKey      = bsonutil.MustHaveTag(PerformanceResultInfo{}, "Tags")
	perfResultInfoArgumentsKey = bsonutil.MustHaveTag(PerformanceResultInfo{}, "Arguments")
	perfResultInfoSchemaKey    = bsonutil.MustHaveTag(PerformanceResultInfo{}, "Schema")
)

// ID creates a unique hash for a performance result.
func (id *PerformanceResultInfo) ID() string {
	var hash hash.Hash

	if id.Schema == 0 {
		// This hash does not include order because it was added as a
		// field after data existed in the database. The order field
		// does not affect uniqueness but will be added in later schema
		// versions.
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

// PerformanceResults describes a set of performance results, typically related
// by some criteria.
type PerformanceResults struct {
	Results   []PerformanceResult `bson:"results"`
	env       cedar.Environment
	populated bool
}

// PerfFindOptions describe the search criteria for the Find function on
// PerformanceResults.
type PerfFindOptions struct {
	Interval    util.TimeRange
	Info        PerformanceResultInfo
	MaxDepth    int
	GraphLookup bool
	Limit       int
	Variant     string
	Sort        []string
}

// Setup sets the environment for the performance results. The environment is
// required for numerous functions on PerformanceResults.
func (r *PerformanceResults) Setup(e cedar.Environment) { r.env = e }

// IsNil returns if the performance results are populated or not.
func (r *PerformanceResults) IsNil() bool { return r.Results == nil }

// Find returns the performance results that are started/completed and matching
// the given criteria.
func (r *PerformanceResults) Find(options PerfFindOptions) error {
	conf, session, err := cedar.GetSessionWithConfig(r.env)
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
	query := session.DB(conf.DatabaseName).C(perfResultCollection).Find(search)
	if options.Limit > 0 {
		query = query.Limit(options.Limit)
	}
	if options.Sort != nil {
		query = query.Sort(options.Sort...)
	} else {
		query = query.Sort("-" + perfCreatedAtKey)
	}
	err = query.All(&r.Results)
	if err != nil {
		return errors.WithStack(err)
	}
	if db.ResultsNotFound(err) {
		return nil
	}
	if options.Info.Parent != "" && len(r.Results) > 0 && options.MaxDepth != 0 {
		// i.e. the parent fits the search criteria
		if options.GraphLookup {
			err = r.findAllChildrenGraphLookup(options.Info.Parent, options.MaxDepth-1, options.Info.Tags)
		} else {
			err = r.findAllChildren(options.Info.Parent, options.MaxDepth)
		}
	}
	if err != nil {
		return errors.WithStack(err)
	}
	if db.ResultsNotFound(errors.Cause(err)) {
		return nil
	}
	r.populated = true

	return nil
}

func (r *PerformanceResults) createFindQuery(options PerfFindOptions) map[string]interface{} {
	search := bson.M{
		"created_at":   bson.M{"$gte": options.Interval.StartAt},
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
		delete(search, "created_at")
		delete(search, "completed_at")
	} else {
		search[bsonutil.GetDottedKeyName("info", "mainline")] = true
	}
	if options.Info.Execution != 0 {
		search[bsonutil.GetDottedKeyName("info", "execution")] = options.Info.Execution
	}
	if options.Info.TestName != "" {
		search[bsonutil.GetDottedKeyName("info", "test_name")] = options.Info.TestName
	}
	if options.Info.Trial != 0 {
		search[bsonutil.GetDottedKeyName("info", "trial")] = options.Info.Trial
	}
	if len(options.Info.Tags) > 0 {
		search[bsonutil.GetDottedKeyName("info", "tags")] = bson.M{"$in": options.Info.Tags}
	}
	if options.Variant != "" {
		search[bsonutil.GetDottedKeyName("info", "variant")] = options.Variant
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

// all children of parent are recursively added to r.Results
func (r *PerformanceResults) findAllChildren(parent string, depth int) error {
	if depth == 0 {
		return nil
	}

	search := bson.M{bsonutil.GetDottedKeyName("info", "parent"): parent}
	conf, session, err := cedar.GetSessionWithConfig(r.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()
	temp := []PerformanceResult{}
	err = session.DB(conf.DatabaseName).C(perfResultCollection).Find(search).All(&temp)
	if err != nil {
		return errors.WithStack(err)
	}

	catcher := grip.NewCatcher()
	for _, result := range temp {
		// look into that parent
		catcher.Add(r.findAllChildren(result.ID, depth-1))
	}

	r.Results = append(r.Results, temp...)

	return errors.WithStack(catcher.Resolve())
}

// all children of parent are added to r.Results using $graphLookup
func (r *PerformanceResults) findAllChildrenGraphLookup(parent string, maxDepth int, tags []string) error {
	conf, session, err := cedar.GetSessionWithConfig(r.env)
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
			"as":               "children",
		},
	}
	if maxDepth >= 0 {
		fields := graphLookup["$graphLookup"].(bson.M)
		fields["maxDepth"] = maxDepth
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

// FindOutdatedRollups returns performance results with missing or outdated
// rollup information for the given `name` and `version`.
func (r *PerformanceResults) FindOutdatedRollups(name string, version int, after time.Time) error {
	conf, session, err := cedar.GetSessionWithConfig(r.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	search := bson.M{
		perfCreatedAtKey: bson.M{"$gt": after},
		bsonutil.GetDottedKeyName(perfArtifactsKey, artifactInfoFormatKey): FileFTDC,
		"$or": []bson.M{
			{
				bsonutil.GetDottedKeyName(perfRollupsKey, perfRollupsStatsKey): bson.M{
					"$elemMatch": bson.M{
						perfRollupValueNameKey:    name,
						perfRollupValueVersionKey: bson.M{"$lt": version},
					},
				},
			},
			{
				bsonutil.GetDottedKeyName(perfRollupsKey, perfRollupsStatsKey, perfRollupValueNameKey): bson.M{"$ne": name},
			},
		},
	}

	return errors.Wrapf(session.DB(conf.DatabaseName).C(perfResultCollection).Find(search).All(&r.Results),
		"problem finding perf results with outdated rollup %s, version %d", name, version)
}
