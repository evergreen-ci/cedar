package model

import (
	dbmodel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/utility"
	"github.com/pkg/errors"
)

// APILog describes metadata for a buildlogger log.
type APILog struct {
	ID          *string            `json:"id,omitempty"`
	Info        APILogInfo         `json:"info,omitempty"`
	CreatedAt   APITime            `json:"created_at"`
	CompletedAt APITime            `json:"completed_at"`
	Duration    float64            `json:"duration_secs"`
	Artifact    APILogArtifactInfo `json:"artifact"`
}

// Import transforms a Log object into an APILog object.
func (apiResult *APILog) Import(i interface{}) error {
	switch l := i.(type) {
	case dbmodel.Log:
		apiResult.ID = utility.ToStringPtr(l.ID)
		apiResult.Info = getLogInfo(l.Info)
		apiResult.CreatedAt = NewTime(l.CreatedAt)
		apiResult.CompletedAt = NewTime(l.CompletedAt)
		apiResult.Duration = l.CompletedAt.Sub(l.CreatedAt).Seconds()
		apiResult.Artifact = getLogArtifactInfo(l.Artifact)
	default:
		return errors.New("incorrect type when fetching converting Log type")
	}
	return nil
}

// APILogInfo describes information unique to a single buildlogger log.
type APILogInfo struct {
	Project     *string           `json:"project,omitempty"`
	Version     *string           `json:"version,omitempty"`
	Variant     *string           `json:"variant,omitempty"`
	TaskName    *string           `json:"task_name,omitempty"`
	TaskID      *string           `json:"task_id,omitempty"`
	Execution   int               `json:"execution"`
	TestName    *string           `json:"test_name,omitempty"`
	Trial       int               `json:"trial"`
	ProcessName *string           `json:"proc_name,omitempty"`
	Format      *string           `json:"format,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Arguments   map[string]string `json:"args,omitempty"`
	ExitCode    int               `json:"exit_code,omitempty"`
}

func getLogInfo(l dbmodel.LogInfo) APILogInfo {
	return APILogInfo{
		Project:     utility.ToStringPtr(l.Project),
		Version:     utility.ToStringPtr(l.Version),
		Variant:     utility.ToStringPtr(l.Variant),
		TaskName:    utility.ToStringPtr(l.TaskName),
		TaskID:      utility.ToStringPtr(l.TaskID),
		Execution:   l.Execution,
		TestName:    utility.ToStringPtr(l.TestName),
		Trial:       l.Trial,
		ProcessName: utility.ToStringPtr(l.ProcessName),
		Format:      utility.ToStringPtr(string(l.Format)),
		Tags:        l.Tags,
		Arguments:   l.Arguments,
		ExitCode:    l.ExitCode,
	}
}

// APILogArtifact describes a bucket of logs stored in some kind of offline
// blob storage. It is the bridge between pail-backed offline log storage and
// the cedar-based log metadata storage. The prefix field indicates the name of
// the "sub-bucket".
type APILogArtifactInfo struct {
	Type    *string           `json:"type"`
	Prefix  *string           `json:"prefix"`
	Version int               `json:"version"`
	Chunks  []APILogChunkInfo `json:"chunks,omitempty"`
}

func getLogArtifactInfo(l dbmodel.LogArtifactInfo) APILogArtifactInfo {
	chunks := make([]APILogChunkInfo, len(l.Chunks))
	for i, chunk := range l.Chunks {
		chunks[i] = getLogChunkInfo(chunk)
	}

	return APILogArtifactInfo{
		Type:    utility.ToStringPtr(string(l.Type)),
		Prefix:  utility.ToStringPtr(l.Prefix),
		Version: l.Version,
		Chunks:  chunks,
	}
}

// APILogChunkInfo describes a chunk of log lines stored in pail-backed offline
// storage.
type APILogChunkInfo struct {
	Key      *string `json:"key"`
	NumLines int     `json:"num_lines"`
	Start    APITime `json:"start"`
	End      APITime `json:"end"`
}

func getLogChunkInfo(l dbmodel.LogChunkInfo) APILogChunkInfo {
	return APILogChunkInfo{
		Key:      utility.ToStringPtr(l.Key),
		NumLines: l.NumLines,
		Start:    NewTime(l.Start),
		End:      NewTime(l.End),
	}
}
