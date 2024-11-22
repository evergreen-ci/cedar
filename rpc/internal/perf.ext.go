package internal

import (
	"time"

	"github.com/evergreen-ci/cedar/model"
	"github.com/mongodb/ftdc/events"
)

type RollupValues []*RollupValue

func (l StorageLocation) Export() model.PailType {
	switch l {
	case StorageLocation_CEDAR_S3, StorageLocation_PROJECT_S3:
		return model.PailS3
	case StorageLocation_LOCAL, StorageLocation_EPHEMERAL:
		return model.PailLocal
	default:
		return ""
	}
}

func (t RollupType) Export() model.MetricType {
	switch t {
	case RollupType_THROUGHPUT:
		return model.MetricTypeThroughput
	case RollupType_LATENCY:
		return model.MetricTypeLatency
	case RollupType_MAX:
		return model.MetricTypeMax
	case RollupType_MEAN:
		return model.MetricTypeMean
	case RollupType_MEDIAN:
		return model.MetricTypeMedian
	case RollupType_MIN:
		return model.MetricTypeMin
	case RollupType_SUM:
		return model.MetricTypeSum
	case RollupType_STANDARD_DEVIATION:
		return model.MetricTypeStdDev
	case RollupType_PERCENTILE_50TH:
		return model.MetricTypePercentile50
	case RollupType_PERCENTILE_80TH:
		return model.MetricTypePercentile80
	case RollupType_PERCENTILE_90TH:
		return model.MetricTypePercentile90
	case RollupType_PERCENTILE_95TH:
		return model.MetricTypePercentile95
	case RollupType_PERCENTILE_99TH:
		return model.MetricTypePercentile99
	default:
		return ""
	}
}

func (m *ResultID) Export() model.PerformanceResultInfo {
	return model.PerformanceResultInfo{
		Project:   m.Project,
		Version:   m.Version,
		Order:     int(m.Order),
		Variant:   m.Variant,
		TaskID:    m.TaskId,
		TaskName:  m.TaskName,
		Execution: int(m.Execution),
		TestName:  m.TestName,
		Parent:    m.Parent,
		Trial:     int(m.Trial),
		Tags:      m.Tags,
		Arguments: m.Arguments,
		Mainline:  m.Mainline,
	}
}

func (a *ArtifactInfo) Export() *model.ArtifactInfo {
	return &model.ArtifactInfo{
		Type:        a.Location.Export(),
		Bucket:      a.Bucket,
		Prefix:      a.Prefix,
		Path:        a.Path,
		Format:      a.Format.Export(),
		Tags:        a.Tags,
		Compression: a.Compression.Export(),
		Schema:      a.Schema.Export(),
		CreatedAt:   a.CreatedAt.AsTime(),
	}
}

func (r *ResultData) Export() *model.PerformanceResult {
	artifacts := []model.ArtifactInfo{}

	for _, a := range r.Artifacts {
		artifacts = append(artifacts, *a.Export())
	}

	result := model.CreatePerformanceResult(r.Id.Export(), artifacts, ExportRollupValues(r.Rollups))

	if r.Id.CreatedAt != nil {
		result.CreatedAt = r.Id.CreatedAt.AsTime()
	} else {
		result.CreatedAt = time.Now()
	}

	return result
}

func (m *MetricsPoint) Export() *events.Performance {
	point := &events.Performance{
		Timestamp: m.Time.AsTime(),
	}

	point.Counters.Size = m.Counters.Size
	point.Counters.Errors = m.Counters.Errors
	point.Counters.Operations = m.Counters.Ops
	point.Gauges.Failed = m.Gauges.Failed
	point.Gauges.Workers = m.Gauges.Workers
	point.Timers.Duration = m.Timers.Duration.AsDuration()
	point.Timers.Total = m.Timers.Total.AsDuration()

	return point
}

func (r *RollupValue) Export() model.PerfRollupValue {
	var value interface{}
	if x, ok := r.Value.(*RollupValue_Int); ok {
		value = x.Int
	} else if x, ok := r.Value.(*RollupValue_Fl); ok {
		value = x.Fl
	}

	return model.PerfRollupValue{
		Name:          r.Name,
		Version:       int(r.Version),
		Value:         value,
		UserSubmitted: r.UserSubmitted,
		MetricType:    r.Type.Export(),
	}
}

func ExportRollupValues(r []*RollupValue) []model.PerfRollupValue {
	perfRollupValues := []model.PerfRollupValue{}
	for _, rollupValue := range r {
		exportedRollup := rollupValue.Export()
		perfRollupValues = append(perfRollupValues, exportedRollup)
	}
	return perfRollupValues
}
