package internal

import "github.com/evergreen-ci/cedar/model"

func (f DataFormat) Export() model.FileDataFormat {
	switch f {
	case DataFormat_FTDC:
		return model.FileFTDC
	case DataFormat_BSON:
		return model.FileBSON
	case DataFormat_CSV:
		return model.FileCSV
	case DataFormat_TEXT:
		return model.FileText
	case DataFormat_JSON:
		return model.FileJSON
	default:
		return model.FileText
	}
}

func (c CompressionType) Export() model.FileCompression {
	switch c {
	case CompressionType_NONE:
		return model.FileUncompressed
	case CompressionType_GZ:
		return model.FileGz
	case CompressionType_TARGZ:
		return model.FileTarGz
	case CompressionType_XZ:
		return model.FileXz
	case CompressionType_ZIP:
		return model.FileZip
	default:
		return model.FileUncompressed
	}
}

func (s SchemaType) Export() model.FileSchema {
	switch s {
	case SchemaType_COLLAPSED_EVENTS:
		return model.SchemaCollapsedEvents
	case SchemaType_INTERVAL_SUMMARIZATION:
		return model.SchemaIntervalSummary
	case SchemaType_HISTOGRAM:
		return model.SchemaHistogram
	default:
		return model.SchemaRawEvents
	}
}
