package model

import (
	"context"
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
	"go.mongodb.org/mongo-driver/mongo/options"
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
	Artifacts            []ArtifactInfo `bson:"artifacts"`
	FailedRollupAttempts int            `bson:"failed_rollup_attempts"`

	Rollups  PerfRollups  `bson:"rollups"`
	Analysis PerfAnalysis `bson:"analysis"`

	env       cedar.Environment
	populated bool
}

var (
	perfIDKey                = bsonutil.MustHaveTag(PerformanceResult{}, "ID")
	perfInfoKey              = bsonutil.MustHaveTag(PerformanceResult{}, "Info")
	perfCreatedAtKey         = bsonutil.MustHaveTag(PerformanceResult{}, "CreatedAt")
	perfCompletedAtKey       = bsonutil.MustHaveTag(PerformanceResult{}, "CompletedAt")
	perfArtifactsKey         = bsonutil.MustHaveTag(PerformanceResult{}, "Artifacts")
	perfFailedRollupAttempts = bsonutil.MustHaveTag(PerformanceResult{}, "FailedRollupAttempts")
	perfRollupsKey           = bsonutil.MustHaveTag(PerformanceResult{}, "Rollups")
	perfAnalysisKey          = bsonutil.MustHaveTag(PerformanceResult{}, "Analysis")
	perfVersionlKey          = bsonutil.MustHaveTag(PerformanceResult{}, "Version")
)

func (info *PerformanceResultInfo) ToPerformanceResultSeriesID() PerformanceResultSeriesID {
	return PerformanceResultSeriesID{
		Project: info.Project,
		Variant: info.Variant,
		Task:    info.TaskName,
		Test:    info.TestName,
	}
}

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
			id:          info.ID(),
			Stats:       append([]PerfRollupValue{}, rollups...),
			ProcessedAt: createdAt,
		},
		populated: true,
		Analysis: PerfAnalysis{
			ChangePoints: []ChangePoint{},
		},
	}
}

// Setup sets the environment for the performance result. The environment is
// required for numerous functions on PerformanceResult.
func (result *PerformanceResult) Setup(e cedar.Environment) { result.env = e }

// IsNil returns if the performance result is populated or not.
func (result *PerformanceResult) IsNil() bool { return !result.populated }

// Find searches the database for the performance result. The environment should
// not be nil.
func (result *PerformanceResult) Find(ctx context.Context) error {
	if result.env == nil {
		return errors.New("cannot find with a nil environment")
	}

	if result.ID == "" {
		result.ID = result.Info.ID()
	}

	result.populated = false
	err := result.env.GetDB().Collection(perfResultCollection).FindOne(ctx, bson.M{"_id": result.ID}).Decode(result)
	if db.ResultsNotFound(err) {
		return errors.New("could not find performance result record in the database")
	} else if err != nil {
		return errors.Wrap(err, "problem finding performance result")
	}

	result.populated = true
	result.Rollups.id = result.ID

	return nil
}

// SaveNew saves a new performance result to the database, if a result with the
// same ID already exists an error is returned. The result should be populated
// and the environment should not be nil.
func (result *PerformanceResult) SaveNew(ctx context.Context) error {
	if !result.populated {
		return errors.New("cannot save unpopulated performance result")
	}
	if result.env == nil {
		return errors.New("cannot save with a nil environment")
	}

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
func (result *PerformanceResult) AppendArtifacts(ctx context.Context, artifacts []ArtifactInfo) error {
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
		"op":           "append artifacts to a performance result",
	})
	if err == nil && updateResult.MatchedCount == 0 {
		err = errors.Errorf("could not find performance result record with id %s in the database", result.ID)
	}

	return errors.Wrapf(err, "problem appending artifacts to performance result with id %s", result.ID)
}

// IncFailedRollupAttempts increments the failed_rollup_attempts field by 1.
// The environment should not be nil.
func (result *PerformanceResult) IncFailedRollupAttempts(ctx context.Context) error {
	if result.env == nil {
		return errors.New("cannot not increment failed rollup attempts with a nil environment")
	}

	if result.ID == "" {
		result.ID = result.Info.ID()
	}

	updateResult, err := result.env.GetDB().Collection(perfResultCollection).UpdateOne(
		ctx,
		bson.M{"_id": result.ID},
		bson.M{"$inc": bson.M{perfFailedRollupAttempts: 1}},
	)
	if err == nil && updateResult.MatchedCount == 0 {
		err = errors.Errorf("could not find performance result record with id %s in the database", result.ID)
	}

	return errors.Wrapf(err, "problem incrementing failed rollup attemps for performance result with id %s", result.ID)
}

// Remove removes the performance result from the database. The environment
// should not be nil.
func (result *PerformanceResult) Remove(ctx context.Context) (int, error) {
	if result.env == nil {
		return -1, errors.New("cannot remove a performance result with a nil environment")
	}

	if result.ID == "" {
		result.ID = result.Info.ID()
	}

	children := PerformanceResults{env: result.env}
	if err := children.findAllChildrenGraphLookup(ctx, result.ID, -1, []string{}); err != nil {
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
	deleteResult, err := result.env.GetDB().Collection(perfResultCollection).DeleteMany(ctx, query)
	grip.DebugWhen(err == nil, message.Fields{
		"collection":   perfResultCollection,
		"id":           result.ID,
		"deleteResult": deleteResult,
		"op":           "remove performance result",
	})
	if err != nil {
		return -1, errors.Wrap(err, "problem removing performance results")
	}
	return int(deleteResult.DeletedCount), nil
}

// Close "closes out" the performance result by populating the completed_at
// field. The envirnment should not be nil.
func (result *PerformanceResult) Close(ctx context.Context, completedAt time.Time) error {
	if result.env == nil {
		return errors.New("cannot close performance result with a nil environment")
	}

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
		err = errors.Errorf("could not find performance result record with id %s in the database", result.ID)
	}

	return errors.Wrapf(err, "problem closing performance result with id %s", result.ID)

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
	perfResultInfoMainlineKey  = bsonutil.MustHaveTag(PerformanceResultInfo{}, "Mainline")
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
func (r *PerformanceResults) Find(ctx context.Context, opts PerfFindOptions) error {
	if r.env == nil {
		return errors.New("cannot find with a nil env")
	}

	search := make(map[string]interface{})
	if opts.Info.Parent != "" { // this is the root node
		search["_id"] = opts.Info.Parent
	} else {
		if opts.Interval.IsZero() || !opts.Interval.IsValid() {
			return errors.New("invalid time range given")
		}
		search = r.createFindQuery(opts)
	}

	r.populated = false
	findOpts := options.Find()
	if opts.Limit > 0 {
		findOpts.SetLimit(int64(opts.Limit))
	}
	if opts.Sort != nil {
		sortKeys := make(bson.D, len(opts.Sort))
		for i, key := range opts.Sort {
			sortKeys[i] = bson.E{key, -1}
		}
		findOpts.SetSort(sortKeys)
	} else {
		findOpts.SetSort(bson.D{{perfCreatedAtKey, -1}})
	}
	it, err := r.env.GetDB().Collection(perfResultCollection).Find(ctx, search, findOpts)
	if err != nil {
		return errors.WithStack(err)
	}
	if err = it.All(ctx, &r.Results); err != nil {
		catcher := grip.NewBasicCatcher()
		catcher.Add(errors.WithStack(err))
		catcher.Add(errors.WithStack(it.Close(ctx)))
		return catcher.Resolve()
	} else if err = it.Close(ctx); err != nil {
		return errors.WithStack(err)
	}

	if opts.Info.Parent != "" && len(r.Results) > 0 && opts.MaxDepth != 0 {
		// i.e. the parent fits the search criteria
		if opts.GraphLookup {
			err = r.findAllChildrenGraphLookup(ctx, opts.Info.Parent, opts.MaxDepth-1, opts.Info.Tags)
		} else {
			err = r.findAllChildren(ctx, opts.Info.Parent, opts.MaxDepth)
		}
		if err != nil {
			return errors.WithStack(err)
		}
	}
	r.populated = true

	return nil
}

func (r *PerformanceResults) createFindQuery(opts PerfFindOptions) map[string]interface{} {
	search := bson.M{
		"created_at":   bson.M{"$gte": opts.Interval.StartAt},
		"completed_at": bson.M{"$lte": opts.Interval.EndAt},
	}

	if opts.Info.Project != "" {
		search[bsonutil.GetDottedKeyName("info", "project")] = opts.Info.Project
	}
	if opts.Info.Version != "" {
		search[bsonutil.GetDottedKeyName("info", "version")] = opts.Info.Version
	}
	if opts.Info.TaskName != "" {
		search[bsonutil.GetDottedKeyName("info", "task_name")] = opts.Info.TaskName
	}
	if opts.Info.TaskID != "" {
		search[bsonutil.GetDottedKeyName("info", "task_id")] = opts.Info.TaskID
		delete(search, "created_at")
		delete(search, "completed_at")
	} else {
		search[bsonutil.GetDottedKeyName("info", "mainline")] = true
	}
	if opts.Info.Execution != 0 {
		search[bsonutil.GetDottedKeyName("info", "execution")] = opts.Info.Execution
	}
	if opts.Info.TestName != "" {
		search[bsonutil.GetDottedKeyName("info", "test_name")] = opts.Info.TestName
	}
	if opts.Info.Trial != 0 {
		search[bsonutil.GetDottedKeyName("info", "trial")] = opts.Info.Trial
	}
	if len(opts.Info.Tags) > 0 {
		search[bsonutil.GetDottedKeyName("info", "tags")] = bson.M{"$in": opts.Info.Tags}
	}
	if opts.Variant != "" {
		search[bsonutil.GetDottedKeyName("info", "variant")] = opts.Variant
	}

	if len(opts.Info.Arguments) > 0 {
		var args []bson.M
		for key, val := range opts.Info.Arguments {
			args = append(args, bson.M{key: val})
		}
		search[bsonutil.GetDottedKeyName("info", "args")] = bson.M{"$in": args}
	}
	return search
}

// all children of parent are recursively added to r.Results
func (r *PerformanceResults) findAllChildren(ctx context.Context, parent string, depth int) error {
	if r.env == nil {
		return errors.New("cannot find children with nil env")
	}

	if depth == 0 {
		return nil
	}

	search := bson.M{bsonutil.GetDottedKeyName("info", "parent"): parent}
	temp := []PerformanceResult{}
	it, err := r.env.GetDB().Collection(perfResultCollection).Find(ctx, search)
	if db.ResultsNotFound(err) {
		return errors.WithStack(it.Close(ctx))
	} else if err != nil {
		return errors.WithStack(err)
	}
	if err = it.All(ctx, &temp); err != nil {
		catcher := grip.NewBasicCatcher()
		catcher.Add(errors.WithStack(err))
		catcher.Add(errors.WithStack(it.Close(ctx)))
		return catcher.Resolve()
	}

	catcher := grip.NewCatcher()
	catcher.Add(it.Close(ctx))
	for _, result := range temp {
		// look into that parent
		catcher.Add(r.findAllChildren(ctx, result.ID, depth-1))
	}

	r.Results = append(r.Results, temp...)

	return errors.WithStack(catcher.Resolve())
}

// all children of parent are added to r.Results using $graphLookup
func (r *PerformanceResults) findAllChildrenGraphLookup(ctx context.Context, parent string, maxDepth int, tags []string) error {
	if r.env == nil {
		return errors.New("cannot find children with nil env")
	}

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
	it, err := r.env.GetDB().Collection(perfResultCollection).Aggregate(ctx, pipeline)
	if err != nil {
		return errors.WithStack(err)
	}

	docs := []struct {
		Children []PerformanceResult `bson:"children,omitempty"`
	}{}
	if err = it.All(ctx, &docs); err != nil {
		catcher := grip.NewBasicCatcher()
		catcher.Add(errors.WithStack(err))
		catcher.Add(errors.WithStack(it.Close(ctx)))
		return catcher.Resolve()
	}
	for _, doc := range docs {
		r.Results = append(r.Results, doc.Children...)
	}

	return errors.WithStack(it.Close(ctx))
}

// FindOutdatedRollups returns performance results with missing or outdated
// rollup information for the given `name` and `version`.
func (r *PerformanceResults) FindOutdatedRollups(ctx context.Context, name string, version int, after time.Time, failureLimit int) error {
	if r.env == nil {
		return errors.New("cannot find outdated rollups with a nil env")
	}

	search := bson.M{
		perfCreatedAtKey: bson.M{"$gt": after},
		bsonutil.GetDottedKeyName(perfArtifactsKey, artifactInfoFormatKey): FileFTDC,
		perfFailedRollupAttempts: bson.M{"$lt": failureLimit},
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
	it, err := r.env.GetDB().Collection(perfResultCollection).Find(ctx, search)
	if db.ResultsNotFound(err) {
		return errors.WithStack(it.Close(ctx))
	} else if err != nil {
		return errors.Wrapf(err, "problem finding performance results with outdated rollup %s, version %d", name, version)
	}
	if err = it.All(ctx, &r.Results); err != nil {
		catcher := grip.NewBasicCatcher()
		catcher.Add(errors.WithStack(err))
		catcher.Add(errors.WithStack(it.Close(ctx)))
		return errors.Wrapf(catcher.Resolve(), "problem finding performance results with outdated rollup %s, version %d", name, version)
	}

	return errors.WithStack(it.Close(ctx))
}
