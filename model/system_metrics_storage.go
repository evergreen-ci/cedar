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
	Prefix  string   `bson:"prefix"`
	Key     []string `bson:"path"`
	Options SystemMetricsArtifactOptions
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
	metricsArtifactInfoTypeKey        = bsonutil.MustHaveTag(SystemMetricsArtifactInfo{}, "Type")
	metricsArtifactInfoPrefixKey      = bsonutil.MustHaveTag(SystemMetricsArtifactInfo{}, "Prefix")
	metricsArtifactInfoKeyKey         = bsonutil.MustHaveTag(SystemMetricsArtifactInfo{}, "Key")
	metricsArtifactInfoSchemaKey      = bsonutil.MustHaveTag(SystemMetricsArtifactInfo{}, "Schema")
	metricsArtifactInfoFormatKey      = bsonutil.MustHaveTag(SystemMetricsArtifactInfo{}, "Format")
	metricsArtifactInfoCompressionKey = bsonutil.MustHaveTag(SystemMetricsArtifactInfo{}, "Compression")
)
