package internal

import (
	"github.com/evergreen-ci/cedar/model"
	"github.com/golang/protobuf/ptypes"
	"github.com/pkg/errors"
)

// Export exports LogFormat to its corresponding LogFormat type in the model
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

// Export exports LogLine to its corresponding LogLine type in the model
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

// Export exports LogInfo to its corresponding LogInfo type in the model
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
		Arguments:   l.Arguments,
		Mainline:    l.Mainline,
	}
}
