package model

import (
	"crypto/sha1"
	"fmt"
	"hash"
	"io"
	"sort"
	"time"

	"github.com/evergreen-ci/sink"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/anser/model"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

const perfResultCollection = "perf_results"

type PerformanceResult struct {
	ID          string              `bson:"_id"`
	Info        PerformanceResultID `bson:"info"`
	CreatedAt   time.Time           `bson:"created_ts"`
	CompletedAt time.Time           `bson:"completed_at"`
	Version     int                 `bson:"version"`

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
	perfIDKey          = bsonutil.MustHaveTag(PerformanceResult{}, "ID")
	perfInfoKey        = bsonutil.MustHaveTag(PerformanceResult{}, "Info")
	perfSourceKey      = bsonutil.MustHaveTag(PerformanceResult{}, "Source")
	perfAuxDataKey     = bsonutil.MustHaveTag(PerformanceResult{}, "AuxilaryData")
	perfDataSummaryKey = bsonutil.MustHaveTag(PerformanceResult{}, "DataSummary")
)

func CreatePerformanceResult(info PerformanceResultID, source []ArtifactInfo) *PerformanceResult {
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

type PerformanceResultID struct {
	Project   string           `bson:"project"`
	Version   string           `bson:"version"`
	TaskName  string           `bson:"task_name"`
	TaskID    string           `bson:"task_id"`
	Execution int              `bson:"execution"`
	TestName  string           `bson:"test_name"`
	Trial     int              `bson:"trial"`
	Parent    string           `bson:"parent"`
	Tags      []string         `bson:"tags"`
	Arguments map[string]int32 `bson:"args"`
	Schema    int              `bson:"schema"`
}

var (
	perfResultInfoProjectKey   = bsonutil.MustHaveTag(PerformanceResultID{}, "Project")
	perfResultInfoVersionKey   = bsonutil.MustHaveTag(PerformanceResultID{}, "Version")
	perfResultInfoTaskNameKey  = bsonutil.MustHaveTag(PerformanceResultID{}, "TaskName")
	perfResultInfoTaskIDKey    = bsonutil.MustHaveTag(PerformanceResultID{}, "TaskID")
	perfResultInfoExecutionKey = bsonutil.MustHaveTag(PerformanceResultID{}, "Execution")
	perfResultInfoTestNameKey  = bsonutil.MustHaveTag(PerformanceResultID{}, "TestName")
	perfResultInfoTrialKey     = bsonutil.MustHaveTag(PerformanceResultID{}, "Trial")
	perfResultInfoParentKey    = bsonutil.MustHaveTag(PerformanceResultID{}, "Parent")
	perfResultInfoTagsKey      = bsonutil.MustHaveTag(PerformanceResultID{}, "Tags")
	perfResultInfoArgumentsKey = bsonutil.MustHaveTag(PerformanceResultID{}, "Arguments")
)

func (id *PerformanceResultID) ID() string {
	var hash hash.Hash

	if id.Schema == 0 {
		hash = sha1.New()
		_, _ = io.WriteString(hash, id.Project)
		_, _ = io.WriteString(hash, id.Version)
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
