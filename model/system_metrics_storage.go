package model

import (
	"github.com/mongodb/anser/bsonutil"
)

// SystemMetricsArtifactInfo describes a bucket of system metrics time-series
// data for a given task execution. It is the bridge between pail-backed
// offline metrics storage and the cedar-based metrics metadata storage.

// The prefix field indicates the name of the "sub-bucket". The top level
// bucket is accesible via the cedar.Environment interface.
//
// The schema field describes the format of the data (raw, collapsed,
// interval summarizations, etc.) while the format field describes the encoding
// of the file.
type SystemMetricsArtifactInfo struct {
	Prefix       string                       `bson:"prefix"`
	MetricChunks map[string]MetricChunks      `bson:"metric_chunks"`
	Options      SystemMetricsArtifactOptions `bson:"options"`
}

// MetricChunks represents the chunks of data for a particular type of metric.
// Chunks are the keys of the data objects in storage and Format indicates
// the storage format of the data.
type MetricChunks struct {
	Chunks []string       `bson:"chunks"`
	Format FileDataFormat `bson:"format"`
}

// SystemMetricsArtifactOptions specifies the artifact options that can be
// specified by the caller during object construction. The schema field
// describes the format of the data (raw, collapsed, interval summarizations,
// etc.).
type SystemMetricsArtifactOptions struct {
	Type        PailType        `bson:"type"`
	Compression FileCompression `bson:"compression"`
	Schema      FileSchema      `bson:"schema"`
}

var (
	metricsArtifactInfoPrefixKey         = bsonutil.MustHaveTag(SystemMetricsArtifactInfo{}, "Prefix")
	metricsArtifactInfoMetricChunksKey   = bsonutil.MustHaveTag(SystemMetricsArtifactInfo{}, "MetricChunks")
	metricsArtifactInfoOptionsKey        = bsonutil.MustHaveTag(SystemMetricsArtifactInfo{}, "Options")
	metricsArtifactOptionsTypeKey        = bsonutil.MustHaveTag(SystemMetricsArtifactOptions{}, "Type")
	metricsArtifactOptionsSchemaKey      = bsonutil.MustHaveTag(SystemMetricsArtifactOptions{}, "Schema")
	metricsArtifactOptionsCompressionKey = bsonutil.MustHaveTag(SystemMetricsArtifactOptions{}, "Compression")
	metricsMetricChunksChunksKey         = bsonutil.MustHaveTag(MetricChunks{}, "Chunks")
	metricsMetricChunksFormatKey         = bsonutil.MustHaveTag(MetricChunks{}, "Format")
)
