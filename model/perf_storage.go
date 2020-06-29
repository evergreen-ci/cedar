package model

import (
	"time"

	"github.com/mongodb/anser/bsonutil"
)

// ArtifactInfo is a type that describes an object in some kind of
// offline storage, and is the bridge between pail-backed
// offline-storage and the cedar-based metadata storage.
//
// The schema field describes the format of the data (raw, collapsed,
// interval summarizations, etc.) while the format field describes the
// encoding of the file.
type ArtifactInfo struct {
	Type        PailType        `bson:"type"`
	Bucket      string          `bson:"bucket"`
	Prefix      string          `bson:"prefix"`
	Path        string          `bson:"path"`
	Format      FileDataFormat  `bson:"format"`
	Compression FileCompression `bson:"compression"`
	Schema      FileSchema      `bson:"schema"`
	Tags        []string        `bson:"tags,omitempty"`
	CreatedAt   time.Time       `bson:"created_at"`
}

var (
	artifactInfoTypeKey        = bsonutil.MustHaveTag(ArtifactInfo{}, "Type")
	artifactInfoPathKey        = bsonutil.MustHaveTag(ArtifactInfo{}, "Path")
	artifactInfoSchemaKey      = bsonutil.MustHaveTag(ArtifactInfo{}, "Schema")
	artifactInfoFormatKey      = bsonutil.MustHaveTag(ArtifactInfo{}, "Format")
	artifactInfoCompressionKey = bsonutil.MustHaveTag(ArtifactInfo{}, "Compression")
	artifactInfoTagsKey        = bsonutil.MustHaveTag(ArtifactInfo{}, "Tags")
	artifactInfoCreatedAtKey   = bsonutil.MustHaveTag(ArtifactInfo{}, "CreatedAt")
)

// GetDownloadURL returns the link to download an the given artifact.
func (a *ArtifactInfo) GetDownloadURL() string {
	return a.Type.GetDownloadURL(a.Bucket, a.Prefix, a.Path)
}
