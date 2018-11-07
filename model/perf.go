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
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
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
	Source []ArtifactInfo `bson:"source_info,omitempty"`

	// Tests may collect and upload other data (e.g. ftdc data
	// from servers, and we want to be able to track it here,
	// particularly for use in auxiliary reporting and post-hoc
	// analysis, but worth tracking seperatly from the primary results)
	AuxilaryData []ArtifactInfo `bson:"aux_data,omitempty"`

	// Total represents the sum of all events, and used for tests
	// that report a single summarized event rather than a
	// sequence of timeseries points. Is omitted except when
	// provided by the test.
	Total *PerformancePoint `bson:"total,omitempty"`

	Rollups *PerfRollups `bson:"rollups,omitempty"`

	env       sink.Environment
	populated bool
}

var (
	perfIDKey       = bsonutil.MustHaveTag(PerformanceResult{}, "ID")
	perfInfoKey     = bsonutil.MustHaveTag(PerformanceResult{}, "Info")
	perfSourceKey   = bsonutil.MustHaveTag(PerformanceResult{}, "Source")
	perfAuxDataKey  = bsonutil.MustHaveTag(PerformanceResult{}, "AuxilaryData")
	perfRollupsKey  = bsonutil.MustHaveTag(PerformanceResult{}, "Rollups")
	perfTotalKey    = bsonutil.MustHaveTag(PerformanceResult{}, "Total")
	perfVersionlKey = bsonutil.MustHaveTag(PerformanceResult{}, "Version")
)

func CreatePerformanceResult(info PerformanceResultInfo, source []ArtifactInfo) *PerformanceResult {
	return &PerformanceResult{
		ID:        info.ID(),
		Source:    source,
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
		io.WriteString(hash, id.Project)
		io.WriteString(hash, id.Version)
		io.WriteString(hash, id.TaskName)
		io.WriteString(hash, id.TaskID)
		io.WriteString(hash, fmt.Sprint(id.Execution))
		io.WriteString(hash, id.TestName)
		io.WriteString(hash, fmt.Sprint(id.Trial))
		io.WriteString(hash, id.Parent)

		sort.Strings(id.Tags)
		for _, str := range id.Tags {
			io.WriteString(hash, str)
		}

		if len(id.Arguments) > 0 {
			args := []string{}
			for k, v := range id.Arguments {
				args = append(args, fmt.Sprintf("%s=%d", k, v))
			}

			sort.Strings(args)
			for _, str := range args {
				io.WriteString(hash, str)
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
	Interval util.TimeRange
	Info     PerformanceResultInfo
}

func (r *PerformanceResults) Setup(e sink.Environment) { r.env = e }

// Returns the PerformanceResults that are started/completed within the given range (if completed).
func (r *PerformanceResults) Find(options PerfFindOptions) error {
	if options.Interval.IsZero() || !options.Interval.IsValid() {
		return errors.New("invalid time range given")
	}

	conf, session, err := sink.GetSessionWithConfig(r.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()
	search := r.createFindQuery(options)
	if options.Info.Parent != "" { // this is the root node
		search["_id"] = options.Info.Parent
	}

	r.populated = false
	err = session.DB(conf.DatabaseName).C(perfResultCollection).Find(search).All(&r.Results)
	if options.Info.Parent != "" && len(r.Results) > 0 { // i.e. the parent fits the search criteria
		err = r.findAllChildren(options.Info.Parent)
	}
	if err != nil && !db.ResultsNotFound(err) {
		return errors.WithStack(err)
	}
	r.populated = true
	return nil
}

func (r *PerformanceResults) createFindQuery(options PerfFindOptions) map[string]interface{} {
	search := db.Document{
		"created_ts":   db.Document{"$gte": options.Interval.StartAt},
		"completed_at": db.Document{"$lte": options.Interval.EndAt},
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
		search[bsonutil.GetDottedKeyName("info", "tags")] =
			db.Document{"$in": options.Info.Tags}
	}
	if len(options.Info.Arguments) > 0 {
		var args []db.Document
		for key, val := range options.Info.Arguments {
			args = append(args, db.Document{key: val})
		}
		search[bsonutil.GetDottedKeyName("info", "args")] = db.Document{"$in": args}
	}
	return search
}

// All children of parent are recursively added to r.Results
func (r *PerformanceResults) findAllChildren(parent string) error {
	search := db.Document{bsonutil.GetDottedKeyName("info", "parent"): parent}
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
		err = r.findAllChildren(result.ID)
	}
	return err
}

////////////////////////////////////////////////////////////////////////
//
// Performance Data Roll up Processing

// PerformancePoint represents the "required" data that we read out of
// the stream of events from the tests. The values in these streams
// are combined in roll ups, though the raw data streams may have more
// data.
//
// If you want to add a new data point to one of these structures, you
// should add the fields to the appropraite sub-structure, to all of
// the subsequent rollup tools (e.g. the rest of the methods in this
// file), as well as to the protocol buffer in the top level of this
// repository.
type PerformancePoint struct {
	// Each point must report the timestamp of its collection.
	Timestamp time.Time `bson:"ts" json:"ts" yaml:"ts"`

	// Counters refer to the number of operations/events or total
	// of things since the last collection point. These values are
	// used in computing various kinds of throughput measurements.
	Counters struct {
		Operations int64 `bson:"ops" json:"ops" yaml:"ops"`
		Size       int64 `bson:"size" json:"size" yaml:"size"`
		Errors     int64 `bson:"errors" json:"errors" yaml:"errors"`
	} `bson:"counters" json:"counters" yaml:"counters"`

	// Timers refers to all of the timing data for this event. In
	// general Duration+Waiting should equal the time since the
	// last data point.
	Timers struct {
		Duration time.Duration `bson:"dur" json:"dur" yaml:"dur"`
		Waiting  time.Duration `bson:"wait" json:"wait" yaml:"wait"`
	} `bson:"timers" json:"timers" yaml:"timers"`

	// The State document holds simple counters that aren't
	// expected to change between points, but are useful as
	// annotations of the experiment or descriptions of events in
	// the system configuration.
	State struct {
		Workers int64 `bson:"workers" json:"workers" yaml:"workers"`
		Failed  bool  `bson:"failed" json:"failed" yaml:"failed"`
	} `bson:"state" json:"state" yaml:"state"`
}
