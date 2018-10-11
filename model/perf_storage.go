package model

import (
	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/pail"
	"github.com/mongodb/anser/bsonutil"
)

type PailType string

const (
	PailS3           PailType = "s3"
	PailLegacyGridFS          = "gridfs-legacy"
)

func (t PailType) Create(env sink.Environment) pail.Bucket { return nil }

type FileDataFormat string

const (
	FileFTDC PerformanceDataFormat = "ftdc"
	FileBSON                       = "bson"
	FileJSON                       = "json"
	FileCSV                        = "csv"
)

type FileCompression string

const (
	FileUncompressed FileCompression = "none"
	FileTarGz                        = "targz"
	FileZip                          = "zip"
	FileGz                           = "gz"
	FileXz                           = "xz"
)

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
