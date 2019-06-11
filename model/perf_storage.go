package model

import (
	"fmt"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/pail"
	"github.com/mongodb/anser/bsonutil"
	"github.com/pkg/errors"
)

type PailType string

const (
	PailS3           PailType = "s3"
	PailLegacyGridFS PailType = "gridfs-legacy"
	PailGridFS       PailType = "gridfs"
	PailLocal        PailType = "local"

	defaultS3Region = "us-east-1"
)

func (t PailType) Create(env cedar.Environment, bucket, prefix string) (pail.Bucket, error) {
	var b pail.Bucket
	var err error
	ctx, cancel := env.Context()
	defer cancel()

	switch t {
	case PailS3:
		conf := &CedarConfig{}
		conf.Setup(env)
		if err = conf.Find(); err != nil {
			return nil, errors.Wrap(err, "problem getting application configuration")
		}

		opts := pail.S3Options{
			Name:        bucket,
			Prefix:      prefix,
			Region:      defaultS3Region,
			Credentials: pail.CreateAWSCredentials(conf.S3Bucket.AWSKey, conf.S3Bucket.AWSSecret, ""),
		}
		b, err = pail.NewS3Bucket(opts)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	case PailLegacyGridFS, PailGridFS:
		client := env.GetClient()
		conf := env.GetConf()

		opts := pail.GridFSOptions{
			Database: conf.DatabaseName,
			Prefix:   bucket,
		}
		b, err = pail.NewGridFSBucketWithClient(ctx, client, opts)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	case PailLocal:
		opts := pail.LocalOptions{
			Path: bucket,
		}
		b, err = pail.NewLocalBucket(opts)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	default:
		return nil, errors.New("not implemented")
	}

	if err = b.Check(ctx); err != nil {
		return nil, errors.WithStack(err)
	}
	return b, nil
}

func (t PailType) GetDownloadURL(bucket, prefix, path string) string {
	switch t {
	case PailS3:
		return fmt.Sprintf(
			"https://%s.s3.amazonaws.com/%s",
			bucket,
			prefix+"/"+path,
		)
	default:
		return ""
	}
}

type FileDataFormat string

const (
	FileFTDC FileDataFormat = "ftdc"
	FileBSON FileDataFormat = "bson"
	FileJSON FileDataFormat = "json"
	FileCSV  FileDataFormat = "csv"
	FileText FileDataFormat = "text"
)

func (ff FileDataFormat) Validate() error {
	switch ff {
	case FileFTDC, FileBSON, FileJSON, FileCSV:
		return nil
	default:
		return errors.New("invalid data format")
	}
}

type FileCompression string

const (
	FileUncompressed FileCompression = "none"
	FileTarGz        FileCompression = "targz"
	FileZip          FileCompression = "zip"
	FileGz           FileCompression = "gz"
	FileXz           FileCompression = "xz"
)

func (fc FileCompression) Validate() error {
	switch fc {
	case FileUncompressed, FileTarGz, FileZip, FileGz, FileXz:
		return nil
	default:
		return errors.New("invalid compression format")
	}

}

type FileSchema string

const (
	SchemaRawEvents       FileSchema = "raw-events"
	SchemaCollapsedEvents FileSchema = "collapsed-events"
	SchemaIntervalSummary FileSchema = "interval-summarization"
	SchemaHistogram       FileSchema = "histogram"
)

func (fs FileSchema) Validate() error {
	switch fs {
	case SchemaRawEvents, SchemaIntervalSummary, SchemaCollapsedEvents, SchemaHistogram:
		return nil
	default:
		return errors.New("invalid schema specified")
	}

}

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
	artifactInfoSchmeaKey      = bsonutil.MustHaveTag(ArtifactInfo{}, "Schema")
	artifactInfoFormatKey      = bsonutil.MustHaveTag(ArtifactInfo{}, "Format")
	artifactInfoCompressionKey = bsonutil.MustHaveTag(ArtifactInfo{}, "Compression")
	artifactInfoTagsKey        = bsonutil.MustHaveTag(ArtifactInfo{}, "Tags")
	artifactInfoCreatedAtKey   = bsonutil.MustHaveTag(ArtifactInfo{}, "CreatedAt")
)

func (a *ArtifactInfo) GetDownloadURL() string {
	return a.Type.GetDownloadURL(a.Bucket, a.Prefix, a.Path)
}
