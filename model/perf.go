package model

import (
	"bytes"
	"crypto/sha256"
	"fmt"
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
	ID   string              `bson:"_id"`
	Info PerformanceResultID `bson:"info"`

	// The source timeseries data is stored in a remote location,
	// we'll probably need to store an identifier so we know which
	// service to use to access that data. We'd then summarize
	// that data and store it in the document.
	SourcePath string `bson:"source_path"`

	// These should be keyed on implementations of the pail/Bucket
	// interface.
	SourceType string `bson:"source_type"`

	DataSummary *PerformanceMetricSummary `bson:"summary,omitempty"`

	// Samples must be collected at a fixed interval in order for
	// the math that we do on the aggregate values to make sense.
	SampleFrequency time.Duration `bson:"sample_frequencey"`

	env       sink.Environment
	populated bool
}

var (
	perfIDKey              = bsonutil.MustHaveTag(PerformanceResult{}, "ID")
	perfInfoKey            = bsonutil.MustHaveTag(PerformanceResult{}, "Info")
	perfSourcePathKey      = bsonutil.MustHaveTag(PerformanceResult{}, "SourcePath")
	perfDataSummaryKey     = bsonutil.MustHaveTag(PerformanceResult{}, "DataSummary")
	perfSampleFrequencyKey = bsonutil.MustHaveTag(PerformanceResult{}, "SampleFrequency")
)

func CreatePerformanceResult(info PerformanceResultID, path string, frequency time.Duration) *PerformanceResult {
	return &PerformanceResult{
		ID:              info.ID(),
		SourcePath:      path,
		SampleFrequency: frequency,
		Info:            info,
		populated:       true,
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
	buf := &bytes.Buffer{}
	buf.WriteString(id.TaskName)
	buf.WriteString(fmt.Sprint(id.Execution))
	buf.WriteString(id.TestName)
	buf.WriteString(id.Parent)

	sort.Strings(id.Tags)
	for _, str := range id.Tags {
		buf.WriteString(str)
	}

	if len(id.Arguments) > 0 {
		args := []string{}
		for k, v := range id.Arguments {
			args = append(args, fmt.Sprintf("%s=%d", k, v))
		}

		sort.Strings(args)
		for _, str := range args {
			buf.WriteString(str)
		}
	}

	hash := sha256.New()

	return string(hash.Sum(buf.Bytes()))
}

type PerformancePoint struct {
	Size      int64         `bson:"size" json:"size" yaml:"size"`
	Count     int64         `bson:"count" json:"count" yaml:"count"`
	Workers   int64         `bson:"workers" json:"workers" yaml:"workers"`
	Duration  time.Duration `bson:"dur" json:"dur" yaml:"dur"`
	Timestamp time.Time     `bson:"ts" json:"ts" yaml:"ts"`
}

type PerformanceStatistics struct {
	size          stats.Float64Data
	count         stats.Float64Data
	workers       stats.Float64Data
	totalDuration time.Duration
	totalCount    int64

	samples int
	window  time.Duration
}

type PerformanceMetricSummary struct {
	Size          float64       `bson:"size"`
	Count         float64       `bson:"count"`
	Workers       float64       `bson:"workers"`
	TotalDuration time.Duration `bson:"total_duration"`
	TotalCount    int64         `bson:"total_count"`

	samples int
	window  time.Duration
}

type PerformanceTimeSeriesTotal struct {
	Size     int64         `bson:"size" json:"size" yaml:"size"`
	Count    int64         `bson:"count" json:"count" yaml:"count"`
	Workers  int64         `bson:"workers" json:"workers" yaml:"workers"`
	Duration time.Duration `bson:"dur" json:"dur" yaml:"dur"`
	Span     time.Duration `bson:"span" json:"span" yaml:"span"`
}

var (
	perfMetricsSummarySizeKey          = bsonutil.MustHaveTag(PerformanceMetricSummary{}, "Size")
	perfMetricsSummaryCountKey         = bsonutil.MustHaveTag(PerformanceMetricSummary{}, "Count")
	perfMetricsSummaryWorkersKey       = bsonutil.MustHaveTag(PerformanceMetricSummary{}, "Workers")
	perfMetricsSummaryTotalDurationKey = bsonutil.MustHaveTag(PerformanceMetricSummary{}, "TotalDuration")
	perfMetricsSummaryTotalCountKey    = bsonutil.MustHaveTag(PerformanceMetricSummary{}, "TotalCount")
)

type PerformanceTimeSeries []PerformancePoint

func (ts PerformanceTimeSeries) Total() PerformanceTimeSeriesTotal {
	out := PerformanceTimeSeriesTotal{}
	var lastPoint time.Time
	for idx, item := range ts {
		if idx == 0 {
			lastPoint = item.Timestamp
		} else {
			out.Span += item.Timestamp.Sub(lastPoint)
			lastPoint = item.Timestamp
		}

		out.Size += item.Size
		out.Count += item.Count
		out.Workers += item.Workers
		out.Duration += item.Duration
	}

	return out
}

func (ts PerformanceTimeSeries) Statistics(dur time.Duration) PerformanceStatistics {
	out := PerformanceStatistics{
		size:    make(stats.Float64Data, len(ts)),
		count:   make(stats.Float64Data, len(ts)),
		workers: make(stats.Float64Data, len(ts)),
		samples: len(ts),
		window:  dur,
	}

	for idx, point := range ts {
		out.size[idx] = float64(point.Size)
		out.count[idx] = float64(point.Count)
		out.workers[idx] = float64(point.Workers)
		out.totalCount += point.Count
		out.totalDuration += point.Duration
	}

	return out
}

func (perf *PerformanceStatistics) Mean() (PerformanceMetricSummary, error) {
	var err error
	catcher := grip.NewBasicCatcher()
	out := PerformanceMetricSummary{
		samples: perf.samples,
		window:  perf.window,
	}
	out.Size, err = stats.Mean(perf.size)
	catcher.Add(err)

	out.Count, err = stats.Mean(perf.count)
	catcher.Add(err)

	out.Workers, err = stats.Mean(perf.workers)
	catcher.Add(err)

	return out, catcher.Resolve()
}

func (perf *PerformanceMetricSummary) ThroughputOps() float64 {
	return perf.Count / float64(perf.window)
}
func (perf *PerformanceMetricSummary) ThroughputData() float64 {
	return perf.Size / float64(perf.window)
}

func (perf *PerformanceMetricSummary) Latency() time.Duration {
	return perf.TotalDuration / time.Duration(perf.TotalCount)
}

func (perf *PerformanceMetricSummary) AdjustedParallelLatency() time.Duration {
	return (perf.TotalDuration / time.Duration(perf.TotalCount)) / time.Duration(perf.Workers)
}

func (perf *PerformanceMetricSummary) AdjustedParallelThroughputOps() float64 {
	return (perf.Count / perf.Workers) / float64(perf.window)
}

func (perf *PerformanceMetricSummary) AdjustedParallelThroughputData() float64 {
	return (perf.Size / perf.Workers) / float64(perf.window)
}
