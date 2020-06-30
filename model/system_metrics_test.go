package model

import (
	"testing"
	"time"

	"github.com/evergreen-ci/utility"
	"github.com/stretchr/testify/assert"
)

func TestCreateSystemMetrics(t *testing.T) {
	expected := getSystemMetrics()
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
		Artifact: SystemMetricsArtifactInfo{
			Type:        PailLocal,
			Prefix:      info.ID(),
			Key:         "system_metrics",
			Format:      FileFTDC,
			Compression: FileUncompressed,
			Schema:      SchemaRawEvents,
		},
	}
}
