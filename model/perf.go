package model

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"sort"
	"time"

	"github.com/mongodb/grip"
	"github.com/montanaflynn/stats"
)

type PerformanceResultID struct {
	TaskName  string
	Execution int
	TestName  string
	Parent    string
	Tags      []string
}

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

	hash := sha256.New()

	return string(hash.Sum(buf.Bytes()))
}

type PerformancePoint struct {
	Size     int64
	Count    int64
	Workers  int64
	Duration time.Duration
}

type PerformanceStatistics struct {
	size     stats.Float64Data
	count    stats.Float64Data
	workers  stats.Float64Data
	duration stats.Float64Data

	samples int
}

type PerformanceMetricSummary struct {
	Size     float64
	Count    float64
	Workers  float64
	Duration time.Duration

	samples int
}

type PerformanceTimeSeries []PerformancePoint

func (ts PerformanceTimeSeries) Statistics() PerformanceStatistics {
	out := PerformanceStatistics{
		size:     make(stats.Float64Data, len(ts)),
		count:    make(stats.Float64Data, len(ts)),
		workers:  make(stats.Float64Data, len(ts)),
		duration: make(stats.Float64Data, len(ts)),
		samples:  len(ts),
	}

	for idx, point := range ts {
		out.size[idx] = float64(point.Size)
		out.count[idx] = float64(point.Count)
		out.workers[idx] = float64(point.Workers)
		out.duration[idx] = float64(point.Duration)
	}

	return out
}

func (perf *PerformanceStatistics) Mean() (PerformanceMetricSummary, error) {
	var err error
	catcher := grip.NewBasicCatcher()
	out := PerformanceMetricSummary{
		samples: perf.samples,
	}
	out.Size, err = stats.Mean(perf.size)
	catcher.Add(err)

	out.Count, err = stats.Mean(perf.count)
	catcher.Add(err)

	out.Workers, err = stats.Mean(perf.workers)
	catcher.Add(err)

	var dur float64
	dur, err = stats.Mean(perf.workers)
	catcher.Add(err)
	out.Duration = time.Duration(dur)

	return out, catcher.Resolve()
}

func (perf *PerformanceMetricSummary) ThroughputOps() float64 {
	return perf.Count / float64(perf.samples)
}
func (perf *PerformanceMetricSummary) ThroughputData() float64 {
	return perf.Size / float64(perf.samples)
}

func (perf *PerformanceMetricSummary) Latency() time.Duration {
	return perf.Duration / time.Duration(perf.samples)
}

func (perf *PerformanceMetricSummary) AdjustedParallelLatency() time.Duration {
	return (perf.Duration / time.Duration(perf.Workers)) / time.Duration(perf.samples)
}

func (perf *PerformanceMetricSummary) AdjustedParallelThroughputOps() float64 {
	return (perf.Count / perf.Workers) / float64(perf.samples)
}

func (perf *PerformanceMetricSummary) AdjustedParallelThroughputData() float64 {
	return (perf.Size / perf.Workers) / float64(perf.samples)
}
