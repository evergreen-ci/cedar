package model

import (
	dbmodel "github.com/evergreen-ci/cedar/model"
	"github.com/pkg/errors"
)

// APILog describes metadata for a buildlogger log.
type APILog struct {
	ID          APIString          `json:"id,omitempty"`
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
		apiResult.ID = ToAPIString(l.ID)
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
	Project     APIString         `json:"project,omitempty"`
	Version     APIString         `json:"version,omitempty"`
	Variant     APIString         `json:"variant,omitempty"`
	TaskName    APIString         `json:"task_name,omitempty"`
	TaskID      APIString         `json:"task_id,omitempty"`
	Execution   int               `json:"execution"`
	TestName    APIString         `json:"test_name,omitempty"`
	Trial       int               `json:"trial"`
	ProcessName APIString         `json:"proc_name,omitempty"`
	Format      APIString         `json:"format,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Arguments   map[string]string `json:"args,omitempty"`
	ExitCode    int               `json:"exit_code,omitempty"`
}

func getLogInfo(l dbmodel.LogInfo) APILogInfo {
	return APILogInfo{
		Project:     ToAPIString(l.Project),
		Version:     ToAPIString(l.Version),
		Variant:     ToAPIString(l.Variant),
		TaskName:    ToAPIString(l.TaskName),
		TaskID:      ToAPIString(l.TaskID),
		Execution:   l.Execution,
		TestName:    ToAPIString(l.TestName),
		Trial:       l.Trial,
		ProcessName: ToAPIString(l.ProcessName),
		Format:      ToAPIString(string(l.Format)),
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
	Type    APIString         `json:"type"`
	Prefix  APIString         `json:"prefix"`
	Version int               `json:"version"`
	Chunks  []APILogChunkInfo `json:"chunks,omitempty"`
}

func getLogArtifactInfo(l dbmodel.LogArtifactInfo) APILogArtifactInfo {
	chunks := make([]APILogChunkInfo, len(l.Chunks))
	for i, chunk := range l.Chunks {
		chunks[i] = getLogChunkInfo(chunk)
	}

	return APILogArtifactInfo{
		Type:    ToAPIString(string(l.Type)),
		Prefix:  ToAPIString(l.Prefix),
		Version: l.Version,
		Chunks:  chunks,
	}
}

// APILogChunkInfo describes a chunk of log lines stored in pail-backed offline
// storage.
type APILogChunkInfo struct {
	Key      APIString `json:"key"`
	NumLines int       `json:"num_lines"`
	Start    APITime   `json:"start"`
	End      APITime   `json:"end"`
}

func getLogChunkInfo(l dbmodel.LogChunkInfo) APILogChunkInfo {
	return APILogChunkInfo{
		Key:      ToAPIString(l.Key),
		NumLines: l.NumLines,
		Start:    NewTime(l.Start),
		End:      NewTime(l.End),
	}
}
