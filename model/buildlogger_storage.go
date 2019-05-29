package main

import (
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/mongodb/anser/bsonutil"
)

// S3 Permissions is a type that describes the object canned ACL from S3.
type S3Permissions string

const (
	Private                S3Permissions = s3.ObjectCannedACLPrivate
	PublicRead             S3Permissions = s3.ObjectCannedACLPublicRead
	PublicReadWrite        S3Permissions = s3.ObjectCannedACLPublicReadWrite
	AuthenticatedRead      S3Permissions = s3.ObjectCannedACLAuthenticatedRead
	AWSExecRead            S3Permissions = s3.ObjectCannedACLAwsExecRead
	BucketOwnerRead        S3Permissions = s3.ObjectCannedACLBucketOwnerRead
	BucketOwnerFullControl S3Permissions = s3.ObjectCannedACLBucketOwnerFullControl
)

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
