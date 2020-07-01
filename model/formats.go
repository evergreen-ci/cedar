package model

import "github.com/pkg/errors"

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
	case FileFTDC, FileBSON, FileJSON, FileCSV, FileText:
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
