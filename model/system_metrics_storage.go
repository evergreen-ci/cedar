package model

import (
	"github.com/mongodb/anser/bsonutil"
)

// SystemMetricsArtifactInfo describes a bucket of system metrics time-series
// data for a given task execution. It is the bridge between
// pail-backed offline metrics storage and the cedar-based metrics metadata storage.
// The prefix field indicates the name of the "sub-bucket". The top level
// bucket is accesible via the cedar.Environment interface.
//
// The schema field describes the format of the data (raw, collapsed,
// interval summarizations, etc.) while the format field describes the
// encoding of the file.
type SystemMetricsArtifactInfo struct {
	Prefix  string                       `bson:"prefix"`
	Chunks  []string                     `bson:"path"`
	Options SystemMetricsArtifactOptions `bson:"options"`
}

// SystemMetricsArtifactOptions specifies the artifact options that
// can be specified by the caller during object construction.
type SystemMetricsArtifactOptions struct {
	Type        PailType        `bson:"type"`
	Format      FileDataFormat  `bson:"format"`
	Compression FileCompression `bson:"compression"`
	Schema      FileSchema      `bson:"schema"`
}

var (
	metricsArtifactInfoPrefixKey         = bsonutil.MustHaveTag(SystemMetricsArtifactInfo{}, "Prefix")
	metricsArtifactInfoChunksKey         = bsonutil.MustHaveTag(SystemMetricsArtifactInfo{}, "Chunks")
	metricsArtifactInfoOptionsKey        = bsonutil.MustHaveTag(SystemMetricsArtifactInfo{}, "Options")
	metricsArtifactOptionsTypeKey        = bsonutil.MustHaveTag(SystemMetricsArtifactOptions{}, "Type")
	metricsArtifactOptionsSchemaKey      = bsonutil.MustHaveTag(SystemMetricsArtifactOptions{}, "Schema")
	metricsArtifactOptionsFormatKey      = bsonutil.MustHaveTag(SystemMetricsArtifactOptions{}, "Format")
	metricsArtifactOptionsCompressionKey = bsonutil.MustHaveTag(SystemMetricsArtifactOptions{}, "Compression")
)
