package model

import (
	"crypto/sha256"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/evergreen-ci/sink"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/anser/model"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/montanaflynn/stats"
	"github.com/pkg/errors"
)

const perfResultCollection = "perf_results"

type PerformanceResult struct {
	ID          string              `bson:"_id"`
	Info        PerformanceResultID `bson:"info"`
	CreatedAt   time.Time           `bson:"created_ts"`
	CompletedAt time.Time           `bson:"completed_at"`

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

	// TODO: we should track means/p90s/etc separately
	// here. perhaps as a map or a struct with omitempty keys.
	DataSummary *PerformanceMetricSummary `bson:"summary,omitempty"`

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

func CreatePerformanceResult(info PerformanceResultID, source PerformanceSourceInfo) *PerformanceResult {
	return &PerformanceResult{
		ID:        info.ID(),
		Source:    []PerformanceSourceInfo{source},
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
	TaskName  string           `bson:"task_name"`
	Execution int              `bson:"execution"`
	TestName  string           `bson:"test_name"`
	Parent    string           `bson:"parent"`
	Tags      []string         `bson:"tags"`
	Arguments map[string]int32 `bson:"args"`
}

var (
	perfResultInfoTaskNameKey  = bsonutil.MustHaveTag(PerformanceResultID{}, "TaskName")
	perfResultInfoTestNameKey  = bsonutil.MustHaveTag(PerformanceResultID{}, "TestName")
	perfResultInfoExecutionKey = bsonutil.MustHaveTag(PerformanceResultID{}, "Execution")
	perfResultInfoParentKey    = bsonutil.MustHaveTag(PerformanceResultID{}, "Parent")
	perfResultInfoTagsKey      = bsonutil.MustHaveTag(PerformanceResultID{}, "Tags")
	perfResultInfoArgumentsKey = bsonutil.MustHaveTag(PerformanceResultID{}, "Arguments")
)

func (id *PerformanceResultID) ID() string {
	hash := sha256.New()

	io.WriteString(hash, id.TaskName)
	io.WriteString(hash, fmt.Sprint(id.Execution))
	io.WriteString(hash, id.TestName)
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

// PerformanceStatistics is an intermediate form used to calculate
// statistics from a sequence of reports. It is never persisted to the
// database, but provides access to application level aggregations.
type PerformanceStatistics struct {
	counters struct {
		operations stats.Float64Data
		size       stats.Float64Data
		errors     stats.Float64Data
	}

	totalCount struct {
		operations int64
		size       int64
		errors     int64
	}

	timers struct {
		duration stats.Float64Data
		waiting  stats.Float64Data
	}

	totalTime struct {
		duration time.Duration
		waiting  time.Duration
	}

	state struct {
		workers stats.Float64Data
		failed  bool
	}

	samples int
	span    time.Duration
}

// PerforamcneMetricSummary reflects a specific kind of summation,
// (e.g. means/percentiles, etc.) and may be saved to the database as
// a kind of rollup value. This type also provides methods for
// caclulating
type PerformanceMetricSummary struct {
	Counters struct {
		Operations float64 `bson:"ops" json:"ops" yaml:"ops"`
		Size       float64 `bson:"size" json:"size" yaml:"size"`
		Errors     float64 `bson:"errors" json:"errors" yaml:"errors"`
	} `bson:"counters" json:"counters" yaml:"counters"`

	TotalCount struct {
		Operations int64 `bson:"ops" json:"ops" yaml:"ops"`
		Size       int64 `bson:"size" json:"size" yaml:"size"`
		Errors     int64 `bson:"errors" json:"errors" yaml:"errors"`
	} `bson:"total_count" json:"total_count" yaml:"total_count"`

	Timers struct {
		Duration float64 `bson:"dur" json:"dur" yaml:"dur"`
		Waiting  float64 `bson:"wait" json:"wait" yaml:"wait"`
	} `bson:"timers" json:"timers" yaml:"timers"`

	TotalTime struct {
		Duration time.Duration `bson:"dur" json:"dur" yaml:"dur"`
		Waiting  time.Duration `bson:"wait" json:"wait" yaml:"wait"`
	} `bson:"timers" json:"timers" yaml:"timers"`

	State struct {
		Workers float64 `bson:"workers" json:"workers" yaml:"workers"`
		Failed  bool    `bson:"failed" json:"failed" yaml:"failed"`
	} `bson:"state" json:"state" yaml:"state"`

	Span    time.Duration `bson:"span" json:"span" yaml:"span"`
	Samples int           `bson:"samples" json:"samples" yaml:"samples"`
}

// PerformanceTimeSeries provides an expanded, in-memory value
// reflecting the results of a single value. These series reflect,
// generally the data stored in the FTDC chunks that are persisted in
// off-line storage. While the FTDC data often requires additional data
// than a sequence of points, these series are the basis of the
// reporting that this application does across test values.
type PerformanceTimeSeries []PerformancePoint

// Statistics converts a a series into an intermediate format that we
// can use to calculate means/totals/etc.
func (ts PerformanceTimeSeries) Statistics() PerformanceStatistics {
	out := PerformanceStatistics{
		samples: len(ts),
	}

	out.counters.operations = make(stats.Float64Data, len(ts))
	out.counters.size = make(stats.Float64Data, len(ts))
	out.counters.errors = make(stats.Float64Data, len(ts))
	out.state.workers = make(stats.Float64Data, len(ts))

	var lastPoint time.Time

	for idx, point := range ts {
		if idx == 0 {
			lastPoint = point.Timestamp
		} else if point.Timestamp.Before(lastPoint) {
			panic("data is not valid")
		} else {
			out.span += point.Timestamp.Sub(lastPoint)
			lastPoint = point.Timestamp
		}

		out.counters.operations[idx] = float64(point.Counters.Operations)
		out.counters.size[idx] = float64(point.Counters.Size)
		out.counters.errors[idx] = float64(point.Counters.Errors)
		out.state.workers[idx] = float64(point.State.Workers)

		out.totalCount.errors += point.Counters.Errors
		out.totalCount.operations += point.Counters.Operations
		out.totalCount.size += point.Counters.Size

		if point.State.Failed {
			out.state.failed = true
		}

		// handle time differently. negative duration values
		// should always be ignored.
		if point.Timers.Duration > 0 {
			out.timers.duration[idx] = float64(point.Timers.Duration)
			out.totalTime.duration += point.Timers.Duration
		}
		if point.Timers.Waiting > 0 {
			out.timers.waiting[idx] = float64(point.Timers.Waiting)
			out.totalTime.waiting += point.Timers.Duration
		}
	}

	return out
}

func (perf *PerformanceStatistics) Mean() (PerformanceMetricSummary, error) {
	var err error
	catcher := grip.NewBasicCatcher()
	out := PerformanceMetricSummary{
		Samples: perf.samples,
		Span:    perf.span,
	}

	out.TotalTime.Duration = perf.totalTime.duration
	out.TotalTime.Waiting = perf.totalTime.waiting
	out.TotalCount.Errors = perf.totalCount.errors
	out.TotalCount.Size = perf.totalCount.size
	out.TotalCount.Operations = perf.totalCount.operations

	out.State.Failed = perf.state.failed

	out.Counters.Size, err = perf.counters.size.Mean()
	catcher.Add(err)

	out.Counters.Operations, err = perf.counters.operations.Mean()
	catcher.Add(err)

	out.Counters.Errors, err = perf.counters.errors.Mean()
	catcher.Add(err)

	out.State.Workers, err = perf.state.workers.Mean()
	catcher.Add(err)

	out.Timers.Duration, err = perf.timers.duration.Mean()
	catcher.Add(err)

	out.Timers.Waiting, err = perf.timers.waiting.Mean()
	catcher.Add(err)

	return out, catcher.Resolve()
}

func (perf *PerformanceStatistics) P90() (PerformanceMetricSummary, error) {
	return perf.percentile(90.0)
}

func (perf *PerformanceStatistics) P50() (PerformanceMetricSummary, error) {
	return perf.percentile(50.0)
}

func (perf *PerformanceStatistics) percentile(pval float64) (PerformanceMetricSummary, error) {
	var err error
	catcher := grip.NewBasicCatcher()
	out := PerformanceMetricSummary{
		Samples: perf.samples,
		Span:    perf.span,
	}

	out.State.Failed = perf.state.failed
	out.TotalTime.Duration = perf.totalTime.duration
	out.TotalTime.Waiting = perf.totalTime.waiting
	out.TotalCount.Errors = perf.totalCount.errors
	out.TotalCount.Size = perf.totalCount.size
	out.TotalCount.Operations = perf.totalCount.operations

	out.Counters.Size, err = perf.counters.size.Percentile(pval)
	catcher.Add(err)

	out.Counters.Operations, err = perf.counters.operations.Percentile(pval)
	catcher.Add(err)

	out.Counters.Errors, err = perf.counters.errors.Percentile(pval)
	catcher.Add(err)

	out.State.Workers, err = perf.state.workers.Percentile(pval)
	catcher.Add(err)

	out.Timers.Duration, err = perf.timers.duration.Percentile(pval)
	catcher.Add(err)

	out.Timers.Waiting, err = perf.timers.waiting.Percentile(pval)
	catcher.Add(err)

	return out, catcher.Resolve()
}

func (perf *PerformanceMetricSummary) ThroughputOps() float64 {
	return perf.Counters.Operations / float64(perf.Span)
}

func (perf *PerformanceMetricSummary) ThroughputData() float64 {
	return perf.Counters.Size / float64(perf.Span)
}

func (perf *PerformanceMetricSummary) ErrorRate() float64 {
	return perf.Counters.Errors / float64(perf.Span)
}

func (perf *PerformanceMetricSummary) Latency() time.Duration {
	return perf.TotalTime.Duration / time.Duration(perf.TotalCount.Operations)
}
