package model

import (
	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/pail"
	"github.com/mongodb/anser/bsonutil"
	"github.com/pkg/errors"
)

type PailType string

const (
	PailS3           PailType = "s3"
	PailLegacyGridFS          = "gridfs-legacy"
)

func (t PailType) Create(env sink.Environment) (pail.Bucket, error) {
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

type PerformanceSourceInfo struct {
	Type        PailType        `bson:"type"`
	Path        string          `bson:"path"`
	Format      FileDataFormat  `bson:"format"`
	Compression FileCompression `bson:"compression"`
}

var (
	perfSourceInfoTypeKey        = bsonutil.MustHaveTag(PerformanceSourceInfo{}, "Type")
	perfSourceInfoPathKey        = bsonutil.MustHaveTag(PerformanceSourceInfo{}, "Path")
	perfSourceInfoFormatKey      = bsonutil.MustHaveTag(PerformanceSourceInfo{}, "Format")
	perfSourceInfoCompressionKey = bsonutil.MustHaveTag(PerformanceSourceInfo{}, "Compression")
)
