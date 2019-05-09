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
	"github.com/mongodb/anser/model"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
)

const perfResultCollection = "perf_results"

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

func (result *PerformanceResult) Setup(e cedar.Environment) { result.env = e }
func (result *PerformanceResult) IsNil() bool               { return !result.populated }
func (result *PerformanceResult) Find() error {
	conf, session, err := cedar.GetSessionWithConfig(result.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

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

func (result *PerformanceResult) Save() error {
	if !result.populated {
		return errors.New("cannot save non-populated result data")
	}

	if result.ID == "" {
		result.ID = result.Info.ID()
		if result.ID == "" {
			return errors.New("cannot save result data without ID")
		}
	}

	conf, session, err := cedar.GetSessionWithConfig(result.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	changeInfo, err := session.DB(conf.DatabaseName).C(perfResultCollection).UpsertId(result.ID, result)
	grip.DebugWhen(err == nil, message.Fields{
		"ns":          model.Namespace{DB: conf.DatabaseName, Collection: perfResultCollection},
		"id":          result.ID,
		"updated":     changeInfo.Updated,
		"upsertedID":  changeInfo.UpsertedId,
		"rollups.len": len(result.Rollups.Stats),
		"artifacts":   len(result.Artifacts),
		"op":          "save perf result",
	})
	return errors.Wrap(err, "problem saving perf result to collection")
}

func (result *PerformanceResult) Remove() (int, error) {
	if result.ID == "" {
		result.ID = result.Info.ID()
		if result.ID == "" {
			return -1, errors.New("cannot remove result data without ID")
		}
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

////////////////////////////////////////////////////////////////////////
//
// Component Types

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

func (id *PerformanceResultInfo) ID() string {
	var hash hash.Hash

	if id.Schema == 0 {
		hash = sha1.New()
		_, _ = io.WriteString(hash, id.Project)
		_, _ = io.WriteString(hash, id.Version)
		_, _ = io.WriteString(hash, id.Order)
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
	env       cedar.Environment
	populated bool
}

type PerfFindOptions struct {
	Interval    util.TimeRange
	Info        PerformanceResultInfo
	MaxDepth    int
	GraphLookup bool
	Limit       int
	Sort        []string
}

func (r *PerformanceResults) Setup(e cedar.Environment) { r.env = e }
func (r *PerformanceResults) IsNil() bool               { return r.Results == nil }

// Returns the PerformanceResults that are started/completed within the given range (if completed).
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
	}
	query = query.Sort("-" + perfCreatedAtKey)
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

// Returns performance results with missing or outdated rollup information for
// the given `name` and `version`.
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
