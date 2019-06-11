package model

import (
	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/pail"
	"github.com/mongodb/anser/bsonutil"
	"github.com/pkg/errors"
)

// LogFormat is a type that describes the format of a log.
type LogFormat string

// Valid log formats.
const (
	LogFormatUnknown LogFormat = "unknown"
	LogFormatText    LogFormat = "text"
	LogFormatJSON    LogFormat = "json"
	LogFormatBSON    LogFormat = "bson"
)

// Validate the log format.
func (lf LogFormat) Validate() error {
	switch lf {
	case LogFormatUnknown, LogFormatText, LogFormatJSON, LogFormatBSON:
		return nil
	default:
		return errors.New("invalid log format type specified")
	}
}

// LogArtifact is a type that describes a sub-bucket of logs stored in s3. It
// is the bridge between S3-based offline log storage and the cedar-based log
// metadata storage. The prefix field indicates the name of the sub-bucket. The
// top level bucket is accesible via the cedar.Environment interface.
type LogArtifactInfo struct {
	Prefix      string             `bson:"prefix"`
	Permissions pail.S3Permissions `bson:"permissions"`
	Version     int                `bson:"version"`
}

var (
	logArtifactInfoPrefixKey      = bsonutil.MustHaveTag(LogArtifactInfo{}, "Prefix")
	logArtifactInfoPermissionsKey = bsonutil.MustHaveTag(LogArtifactInfo{}, "Permissions")
	logArtifactInfoVersionKey     = bsonutil.MustHaveTag(LogArtifactInfo{}, "Version")
)

// Create a pail bucket, backed by S3, using the top level application bucket
// configuration and the prefix and permissions provided in log artifact info.
func (l *LogArtifactInfo) CreateBucket(env cedar.Environment) (pail.Bucket, error) {
	conf := &CedarConfig{}
	conf.Setup(env)
	if err := conf.Find(); err != nil {
		return nil, errors.Wrap(err, "problem getting application configuration")
	}

	opts := pail.S3Options{
		Name:        conf.S3Bucket.BuildLogsBucket,
		Prefix:      l.Prefix,
		Region:      defaultS3Region,
		Credentials: pail.CreateAWSCredentials(conf.S3Bucket.AWSKey, conf.S3Bucket.AWSSecret, ""),
		Permissions: l.Permissions,
	}
	b, err := pail.NewS3Bucket(opts)
	return b, errors.WithStack(err)
}
