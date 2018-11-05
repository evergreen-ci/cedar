package model

import (
	"time"

	"github.com/evergreen-ci/sink"
	"github.com/mongodb/grip"
	"github.com/montanaflynn/stats"
	"github.com/pkg/errors"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

const (
	idField           = "_id"
	rollupsField      = "rollups"
	rollupNameField   = "rollups.name"
	rollupVerField    = "rollups.version"
	newRollupVal      = "rollups.$.val"
	newRollupVer      = "rollups.$.version"
	pushNewRollup     = "$push"
	setExistingRollup = "$set"
)

type PerfRollupValue struct {
	Name    string      `bson:"name"`
	Value   interface{} `bson:"val"`
	Version int         `bson:"version"`
}

type PerfRollups struct {
	// DefaultStats are populated the background processing by
	// calculating the rollups from the input source data.
	DefaultStats []PerfRollupValue `bson:"system"`

	// UserStats would be populated directly by test harnesses and
	// are not calculated by the system at all. We may not need
	// these at all.
	UserStats []PerfRollupValue `bson:"user"`

	// TODO:
	//  - determine if we want user stats
	//  - determine merge strategy for default and user stats
	//  - figure out what else we want to store here: version,
	//    validity, last processed

	ProcessedAt time.Time `bson:"processed_at"`
	Count       int       `bson:"count"`
	Valid       bool      `bson:"valid"`

	dirty     bool
	populated bool
	id        string
	env       sink.Environment
}

type perfRollupEntries struct {
	Rollups []PerfRollupValue `bson:"rollups"`
}

func (v *PerfRollupValue) getIntLong() (int64, error) {
	if val, ok := v.Value.(int64); ok {
		return val, nil
	} else if val, ok := v.Value.(int32); ok {
		return int64(val), nil
	} else if val, ok := v.Value.(int); ok {
		return int64(val), nil
	}
	return 0, errors.Errorf("mismatched type for name %s", v.Name)
}

func (v *PerfRollupValue) getFloat() (float64, error) {
	if val, ok := v.Value.(float64); ok {
		return val, nil
	}
	return 0, errors.Errorf("mismatched type for name %s", v.Name)
}

func (r *PerfRollups) Setup(env sink.Environment) {
	r.env = env
}

func (r *PerfRollups) Add(name string, version int, value interface{}) error {
<<<<<<< HEAD
	if !r.populated {
=======
<<<<<<< HEAD
	if r.populated == false {
=======
	if !r.populated {
>>>>>>> MAKE-499 edit new rollup model
>>>>>>> MAKE-499 edit new rollup model
		return errors.New("rollups have not been populated")
	}
	conf, session, err := sink.GetSessionWithConfig(r.env)
	if err != nil {
		return errors.Wrap(err, "error connecting")
	}
	defer session.Close()
	// 1) If in database, check version and make sure passed in version isn't older (if so, done)
	c := session.DB(conf.DatabaseName).C(perfResultCollection)
	search := bson.M{
		idField:         r.id,
		rollupNameField: name,
	}

	out := perfRollupEntries{}
	rollup := PerfRollupValue{
		Name:    name,
		Version: version,
		Value:   value,
	}
	err = c.Find(search).Select(bson.M{rollupVerField: 1, rollupNameField: 1}).One(&out)
	if err != nil {
		if err != mgo.ErrNotFound {
			return errors.Wrap(err, "error finding entry")
		}
		// entry DNE, add entry
		insert := bson.M{rollupsField: rollup}
		search = bson.M{idField: r.id}
		err = c.Update(search, bson.M{pushNewRollup: insert})
		if err != nil {
			return errors.Wrap(err, "error pushing new entry")
		}
		r.DefaultStats = append(r.DefaultStats, rollup)
		r.Count++
		return nil
	}
	// update existing entry
	for _, entry := range out.Rollups {
		if entry.Name == name {
			if entry.Version > version {
				return errors.New("outdated version")
			}
			break
		}
	}
	update := bson.M{
		newRollupVal: value,
		newRollupVer: version,
	}
<<<<<<< HEAD
	err = c.Update(search, bson.M{setExistingRollup: update})
	if err != nil {
		return errors.Wrap(err, "error updating an existing entry")
=======
<<<<<<< HEAD
	err2 := c.Update(search, bson.M{setExistingRollup: update})
	if err2 != nil {
		return errors.Wrap(err2, "error updating an existing entry")
=======
	err = c.Update(search, bson.M{setExistingRollup: update})
	if err != nil {
		return errors.Wrap(err, "error updating an existing entry")
>>>>>>> MAKE-499 edit new rollup model
>>>>>>> MAKE-499 edit new rollup model
	}
	// update local entry
	for i := range r.DefaultStats {
		if r.DefaultStats[i].Name == name {
			r.DefaultStats[i].Version = version
			r.DefaultStats[i].Value = value
			return nil
		}
	}
	r.DefaultStats = append(r.DefaultStats, rollup)
	r.Count++
	return nil
}

func (r *PerfRollups) GetInt(name string) (int, error) {
	for _, rollup := range r.DefaultStats {
		if rollup.Name == name {
			if val, ok := rollup.Value.(int); ok {
				return val, nil
			} else if val, ok := rollup.Value.(int32); ok {
				return int(val), nil
			} else {
				return 0, errors.Errorf("mismatched type for name %s", name)
			}
		}
	}
	return 0, errors.Errorf("name %s does not exist", name)
}

func (r *PerfRollups) GetInt32(name string) (int32, error) {
	val, err := r.GetInt(name)
	return int32(val), err
}

func (r *PerfRollups) GetInt64(name string) (int64, error) {
	for _, rollup := range r.DefaultStats {
		if rollup.Name == name {
			return rollup.getIntLong()
		}
	}
	return 0, errors.Errorf("name %s does not exist", name)
}

func (r *PerfRollups) GetFloat(name string) (float64, error) {
	for _, rollup := range r.DefaultStats {
		if rollup.Name == name {
			return rollup.getFloat()
		}
	}
	return 0, errors.Errorf("name %s does not exist", name)
}

func (r *PerfRollups) Validate() error {
	if len(r.DefaultStats) != r.Count {
		return errors.New("number of stats and count of stats not equal")
	}
	return nil
}

func (r *PerfRollups) Map() map[string]int64 {
	result := make(map[string]int64)
	for _, rollup := range r.DefaultStats {
		val, err := rollup.getIntLong()
		if err == nil {
			result[rollup.Name] = val
		}
	}
	return result
}

func (r *PerfRollups) MapFloat() map[string]float64 {
	result := make(map[string]float64)
	for _, rollup := range r.DefaultStats {
		if val, err := rollup.getFloat(); err == nil {
			result[rollup.Name] = val
		} else if val, err := rollup.getIntLong(); err == nil {
			result[rollup.Name] = float64(val)
		}
	}
	return result
}

////////////////////////////////////////////////////////////////////////
//
// Legacy Rollup Calculations. TODO: port to new system asap

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
	} `bson:"total_time" json:"total_time" yaml:"total_time"`

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
