package internal

import (
	"time"

	"github.com/evergreen-ci/cedar/model"
	"github.com/golang/protobuf/ptypes"
	"github.com/mongodb/ftdc/events"
	"github.com/pkg/errors"
)

func (l StorageLocation) Export() model.PailType {
	switch l {
	case StorageLocation_GRIDFS:
		return model.PailLegacyGridFS
	case StorageLocation_CEDAR_S3, StorageLocation_PROJECT_S3:
		return model.PailS3
	case StorageLocation_LOCAL:
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
		return model.MetricTypePercentile99
	case RollupType_PERCENTILE_99TH:
		return model.MetricTypePercentile99
	default:
		return ""
	}
}

func (f DataFormat) Export() model.FileDataFormat {
	switch f {
	case DataFormat_FTDC:
		return model.FileFTDC
	case DataFormat_BSON:
		return model.FileBSON
	case DataFormat_CSV:
		return model.FileCSV
	case DataFormat_TEXT:
		return model.FileText
	case DataFormat_JSON:
		return model.FileJSON
	default:
		return model.FileText
	}
}

func (c CompressionType) Export() model.FileCompression {
	switch c {
	case CompressionType_NONE:
		return model.FileUncompressed
	case CompressionType_GZ:
		return model.FileGz
	case CompressionType_TARGZ:
		return model.FileTarGz
	case CompressionType_XZ:
		return model.FileXz
	case CompressionType_ZIP:
		return model.FileZip
	default:
		return model.FileUncompressed
	}

}

func (s SchemaType) Export() model.FileSchema {
	switch s {
	case SchemaType_COLLAPSED_EVENTS:
		return model.SchemaCollapsedEvents
	case SchemaType_INTERVAL_SUMMARIZATION:
		return model.SchemaIntervalSummary
	case SchemaType_HISTOGRAM:
		return model.SchemaHistogram
	default:
		return model.SchemaRawEvents
	}
}

func (m *ResultID) Export() model.PerformanceResultInfo {
	return model.PerformanceResultInfo{
		Project:   m.Project,
		Version:   m.Version,
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

func (a *ArtifactInfo) Export() (*model.ArtifactInfo, error) {
	ts, err := ptypes.Timestamp(a.CreatedAt)
	if err != nil {
		return nil, errors.Wrap(err, "problem converting timestamp value artifact")
	}

	return &model.ArtifactInfo{
		Type:        a.Location.Export(),
		Bucket:      a.Bucket,
		Path:        a.Path,
		Format:      a.Format.Export(),
		Tags:        a.Tags,
		Compression: a.Compression.Export(),
		Schema:      a.Schema.Export(),
		CreatedAt:   ts,
	}, nil
}

func (r *ResultData) Export() (*model.PerformanceResult, error) {
	artifacts := []model.ArtifactInfo{}

	for _, a := range r.Artifacts {
		artifact, err := a.Export()
		if err != nil {
			return nil, errors.Wrap(err, "problem exporting artifacts")
		}
		artifacts = append(artifacts, *artifact)
	}

	result := model.CreatePerformanceResult(r.Id.Export(), artifacts)

	if r.Id.CreatedAt != nil {
		ts, err := ptypes.Timestamp(r.Id.CreatedAt)
		if err != nil {
			return nil, errors.Wrap(err, "problem converting timestamp value artifact")
		}

		result.CreatedAt = ts
	} else {
		result.CreatedAt = time.Now()
	}

	return result, nil
}

func (m *MetricsPoint) Export() (*events.Performance, error) {
	dur, err := ptypes.Duration(m.Timers.Duration)
	if err != nil {
		return nil, errors.Wrap(err, "problem converting duration value")
	}
	total, err := ptypes.Duration(m.Timers.Total)
	if err != nil {
		return nil, errors.Wrap(err, "problem converting duration value")
	}

	ts, err := ptypes.Timestamp(m.Time)
	if err != nil {
		return nil, errors.Wrap(err, "problem converting timestamp value")
	}

	point := &events.Performance{
		Timestamp: ts,
	}

	point.Counters.Size = m.Counters.Size
	point.Counters.Errors = m.Counters.Errors
	point.Counters.Operations = m.Counters.Ops
	point.Gauges.Failed = m.Gauges.Failed
	point.Gauges.Workers = m.Gauges.Workers
	point.Timers.Duration = dur
	point.Timers.Total = total

	return point, nil
}

func (r *RollupValue) Export() model.PerfRollupValue {
	return model.PerfRollupValue{
		Name:          r.Name,
		Version:       int(r.Version),
		Value:         r.Value,
		UserSubmitted: r.UserSubmitted,
		MetricType:    r.Type.Export(),
	}
}
