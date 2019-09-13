package internal

import (
	"github.com/evergreen-ci/cedar/model"
	"github.com/golang/protobuf/ptypes"
	"github.com/pkg/errors"
)

// Export exports LogFormat to the corresponding LogFormat type in the model
// package.
func (l LogFormat) Export() model.LogFormat {
	switch l {
	case LogFormat_LOG_FORMAT_UNKNOWN:
		return model.LogFormatUnknown
	case LogFormat_LOG_FORMAT_TEXT:
		return model.LogFormatText
	case LogFormat_LOG_FORMAT_JSON:
		return model.LogFormatJSON
	case LogFormat_LOG_FORMAT_BSON:
		return model.LogFormatBSON
	default:
		return model.LogFormatUnknown
	}
}

// Export exports LogStorage to the corresponding PailType in the model
// package.
func (l LogStorage) Export() model.PailType {
	switch l {
	case LogStorage_LOG_STORAGE_S3:
		return model.PailS3
	case LogStorage_LOG_STORAGE_GRIDFS:
		return model.PailGridFS
	case LogStorage_LOG_STORAGE_LOCAL:
		return model.PailLocal
	default:
		return model.PailLocal
	}
}

// Export exports LogLine to the corresponding LogLine type in the model
// package.
func (l LogLine) Export() (model.LogLine, error) {
	ts, err := ptypes.Timestamp(l.Timestamp)
	if err != nil {
		return model.LogLine{}, errors.Wrap(err, "problem converting log line timestamp value")
	}

	return model.LogLine{
		Timestamp: ts,
		Data:      l.Data,
	}, nil
}

// Export exports LogInfo to the corresponding LogInfo type in the model
// package.
func (l LogInfo) Export() model.LogInfo {
	return model.LogInfo{
		Project:     l.Project,
		Version:     l.Version,
		Variant:     l.Variant,
		TaskName:    l.TaskName,
		TaskID:      l.TaskId,
		Execution:   int(l.Execution),
		TestName:    l.TestName,
		Trial:       int(l.Trial),
		ProcessName: l.ProcName,
		Format:      l.Format.Export(),
		Tags:        l.Tags,
		Arguments:   l.Arguments,
		Mainline:    l.Mainline,
	}
}
