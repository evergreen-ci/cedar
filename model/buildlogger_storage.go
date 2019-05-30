package model

import (
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/mongodb/anser/bsonutil"
	"github.com/pkg/errors"
)

// S3 Permissions is a type that describes the object canned ACL from S3.
type S3Permissions string

const (
	S3PermissionsPrivate                S3Permissions = s3.ObjectCannedACLPrivate
	S3PermissionsPublicRead             S3Permissions = s3.ObjectCannedACLPublicRead
	S3PermissionsPublicReadWrite        S3Permissions = s3.ObjectCannedACLPublicReadWrite
	S3PermissionsAuthenticatedRead      S3Permissions = s3.ObjectCannedACLAuthenticatedRead
	S3PermissionsAWSExecRead            S3Permissions = s3.ObjectCannedACLAwsExecRead
	S3PermissionsBucketOwnerRead        S3Permissions = s3.ObjectCannedACLBucketOwnerRead
	S3PermissionsBucketOwnerFullControl S3Permissions = s3.ObjectCannedACLBucketOwnerFullControl
)

func (p S3Permissions) Validate() error {
	switch p {
	case S3PermissionsPublicRead, S3PermissionsPublicReadWrite:
		return nil
	case S3PermissionsPrivate, S3PermissionsAuthenticatedRead, S3PermissionsAWSExecRead:
		return nil
	case S3PermissionsBucketOwnerRead, S3PermissionsBucketOwnerFullControl:
		return nil
	default:
		return errors.New("invalid S3 permissions type specified")
	}
}

// LogArtifact is a type that describes a sub-bucket of logs stored in s3. It
// is the bridge between S3-based offline log storage and the cedar-based log
// metadata storage. The prefix field indicates the name of the sub-bucket. The
// top level bucket is accesible via the cedar.Environment interface.
type LogArtifactInfo struct {
	Prefix      string        `bson:"prefix"`
	Permissions S3Permissions `bson:"permissions"`
	Version     int           `bson:"version"`
}

var (
	logArtifactInfoPrefixKey      = bsonutil.MustHaveTag(LogArtifactInfo{}, "Prefix")
	logArtifactInfoPermissionsKey = bsonutil.MustHaveTag(LogArtifactInfo{}, "Permissions")
	logArtifactInfoVersionKey     = bsonutil.MustHaveTag(LogArtifactInfo{}, "Version")
)
