package model

import (
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
)

func (t PailType) Create(env cedar.Environment, bucket string) (pail.Bucket, error) {
	switch t {
	case PailS3:
		opts := pail.S3Options{
			Name: bucket,
		}
		b, err := pail.NewS3Bucket(opts)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		return b, nil
	case PailLegacyGridFS, PailGridFS:
		ctx, cancel := env.Context()
		defer cancel()

		client, err := env.GetClient()
		if err != nil {
			return nil, errors.WithStack(err)
		}

		conf, err := env.GetConf()
		if err != nil {
			return nil, errors.WithStack(err)
		}

		opts := pail.GridFSOptions{
			Database: conf.DatabaseName,
			Prefix:   bucket,
		}
		b, err := pail.NewGridFSBucketWithClient(ctx, client, opts)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		return b, nil
	case PailLocal:
		opts := pail.LocalOptions{
			Path: bucket,
		}
		b, err := pail.NewLocalBucket(opts)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		return b, nil
	default:
		return nil, errors.New("not implemented")
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
