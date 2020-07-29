package internal

import (
	"github.com/evergreen-ci/cedar/model"
)

// Export exports SystemMetricsInfo to the corresponding SystemMetricsInfo
// type in the model package.
func (sm SystemMetricsInfo) Export() model.SystemMetricsInfo {
	return model.SystemMetricsInfo{
		Project:   sm.Project,
		Version:   sm.Version,
		Variant:   sm.Variant,
		TaskName:  sm.TaskName,
		TaskID:    sm.TaskId,
		Execution: int(sm.Execution),
		Mainline:  sm.Mainline,
	}
}

// Export exports SystemMetricsArtifactInfo to the corresponding SystemMetricsInfo
// type in the model package.
func (sm SystemMetricsArtifactInfo) Export() model.SystemMetricsArtifactOptions {
	return model.SystemMetricsArtifactOptions{
		Compression: sm.Compression.Export(),
		Schema:      sm.Schema.Export(),
		Format:      sm.Format.Export(),
	}
}
