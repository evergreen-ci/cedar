package model

import (
	"github.com/evergreen-ci/pail"
	"github.com/mongodb/anser/bsonutil"
	"github.com/pkg/errors"
)

type LogFormat string

const (
	LogFormatUnknown LogFormat = "unknown"
	LogFormatText    LogFormat = "text"
	LogFormatJSON    LogFormat = "json"
	LogFormatBSON    LogFormat = "bson"
)

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
