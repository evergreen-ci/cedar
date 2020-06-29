package model

import (
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/utility"
	"github.com/stretchr/testify/assert"
)

// CreateSystemMetrics is the entry point for creating the metadata for
// system metric time series data for a task execution.
func CreateSystemMetrics(info SystemMetricsInfo, artifact SystemMetricsArtifactInfo) *SystemMetrics {
	return &SystemMetrics{
		ID:        info.ID(),
		Info:      info,
		CreatedAt: time.Now(),
		Artifact: SystemMetricsArtifactInfo{
			Type:        artifact.Type,
			Prefix:      info.ID(),
			Key:         "system_metrics",
			Format:      artifact.Format,
			Compression: artifact.Compression,
			Schema:      artifact.Schema,
		},
		populated: true,
	}
}

// Setup sets the sets the environment for the system metrics object.
// The environment is required for numerous functions on SystemMetrics.
func (sm *SystemMetrics) Setup(e cedar.Environment) { sm.env = e }

// IsNil returns if the system metrics object is populated or not.
func (sm *SystemMetrics) IsNil() bool { return !sm.populated }

func TestCreateSystemMetrics(t *testing.T) {
	expected, _ := getSystemMetrics()
	expected.populated = true
	artifactInfo := expected.Artifact
	artifactInfo.Prefix = ""
	artifactInfo.Key = ""
	actual := CreateSystemMetrics(expected.Info, artifactInfo)
	assert.Equal(t, expected.ID, actual.ID)
	assert.Equal(t, expected.Info, actual.Info)
	assert.Equal(t, expected.Artifact, actual.Artifact)
	assert.True(t, time.Since(actual.CreatedAt) <= time.Second)
	assert.Zero(t, actual.CompletedAt)
	assert.True(t, actual.populated)
}

func getSystemMetrics() *SystemMetrics {
	info := SystemMetricsInfo{
		Project:  utility.RandomString(),
		Version:  utility.RandomString(),
		Variant:  utility.RandomString(),
		TaskName: utility.RandomString(),
		TaskID:   utility.RandomString(),
		Mainline: true,
		Schema:   0,
	}
	return &SystemMetrics{
		ID:          info.ID(),
		Info:        info,
		CreatedAt:   time.Now().Add(-time.Hour).UTC().Round(time.Millisecond),
		CompletedAt: time.Now().UTC().Round(time.Millisecond),
		Artifact: TestResultsArtifactInfo{
			Type:        PailLocal,
			Prefix:      info.ID(),
			Key:         "system_metrics",
			Format:      FileFTDC,
			Compression: FileUncompressed,
			Schema:      SchemaRawEvents,
		},
	}
}
