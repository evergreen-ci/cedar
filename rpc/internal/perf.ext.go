package internal

import (
	"github.com/evergreen-ci/sink/model"
	"github.com/golang/protobuf/ptypes"
	"github.com/pkg/errors"
)

func (l StorageLocation) Export() model.PailType {
	switch l {
	case StorageLocation_GRIDFS:
		return model.PailLegacyGridFS
	case StorageLocation_SINK_S3, StorageLocation_PROJECT_S3:
		return model.PailS3
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

func (m *ResultID) Export() *model.PerformanceResultID {
	return &model.PerformanceResultID{
		TaskName:  m.TaskName,
		Execution: int(m.Execution),
		TestName:  m.TestName,
		Parent:    m.Parent,
		Tags:      m.Tags,
		Arguments: m.Arguments,
	}
}

func (a *ArtifactInfo) Export() *model.ArtifactInfo {
	return &model.ArtifactInfo{
		Type:        a.Location.Export(),
		Bucket:      a.Bucket,
		Path:        a.Path,
		Format:      a.Format.Export(),
		Tags:        a.Tags,
		Compression: a.Compression.Export(),
	}
}

func (m *MetricsSeriesStart) Export() *model.PerformanceResult {
	artifacts := []model.ArtifactInfo{}

	for _, a := range m.Artifacts {
		artifacts = append(artifacts, *a.Export())
	}

	return model.CreatePerformanceResult(*m.Id.Export(), artifacts)
}

func (m *MetricsPoint) Export() (model.PerformancePoint, error) {
	dur, err := ptypes.Duration(m.Timers.Duration)
	if err != nil {
		return model.PerformancePoint{}, errors.Wrap(err, "problem converting duration value")
	}
	wait, err := ptypes.Duration(m.Timers.Waiting)
	if err != nil {
		return model.PerformancePoint{}, errors.Wrap(err, "problem converting duration value")
	}

	ts, err := ptypes.Timestamp(m.Time)
	if err != nil {
		return model.PerformancePoint{}, errors.Wrap(err, "problem converting duration value")
	}

	point := model.PerformancePoint{
		Timestamp: ts,
	}

	point.Counters.Size = m.Counters.Size
	point.Counters.Errors = m.Counters.Errors
	point.Counters.Operations = m.Counters.Ops
	point.State.Failed = m.State.Failed
	point.State.Workers = m.State.Workers
	point.Timers.Duration = dur
	point.Timers.Waiting = wait

	return point, nil
}
