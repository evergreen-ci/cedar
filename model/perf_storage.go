package model

import (
	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/pail"
	"github.com/mongodb/anser/bsonutil"
	"github.com/pkg/errors"
)

type PailType string

const (
	PailS3           PailType = "s3"
	PailLegacyGridFS          = "gridfs-legacy"
)

func (t PailType) Create(env sink.Environment, bucket string) (pail.Bucket, error) {
	switch t {
	case PailS3:
		return nil, errors.New("not implemented")
	case PailLegacyGridFS:
		conf, session, err := sink.GetSessionWithConfig(env)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		opts := pail.GridFSOptions{
			Database: conf.DatabaseName,
			Prefix:   bucket,
		}

		b, err := pail.NewLegacyGridFSBucketWithSession(session, opts)
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
	FileBSON                = "bson"
	FileJSON                = "json"
	FileCSV                 = "csv"
	FileText                = "text"
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
	FileTarGz                        = "targz"
	FileZip                          = "zip"
	FileGz                           = "gz"
	FileXz                           = "xz"
)

func (fc FileCompression) Validate() error {
	switch fc {
	case FileUncompressed, FileTarGz, FileZip, FileGz, FileXz:
		return nil
	default:
		return errors.New("invalid compression format")
	}

}

// ArtifactInfo is a type that describes an object in some kind of
// offline storage, and is the bridge between pail-backed
// offline-storage and the sink-based metadata storage.
type ArtifactInfo struct {
	Type        PailType        `bson:"type"`
	Bucket      string          `bson:"bucket"`
	Path        string          `bson:"path"`
	Format      FileDataFormat  `bson:"format"`
	Compression FileCompression `bson:"compression"`
	Tags        []string        `bson:"tags,omitempty"`
}

var (
	artifactInfoTypeKey        = bsonutil.MustHaveTag(ArtifactInfo{}, "Type")
	artifactInfoPathKey        = bsonutil.MustHaveTag(ArtifactInfo{}, "Path")
	artifactInfoFormatKey      = bsonutil.MustHaveTag(ArtifactInfo{}, "Format")
	artifactInfoCompressionKey = bsonutil.MustHaveTag(ArtifactInfo{}, "Compression")
	artifactInfoTagsKey        = bsonutil.MustHaveTag(ArtifactInfo{}, "Tags")
)
