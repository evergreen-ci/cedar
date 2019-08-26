package model

import (
	"testing"
	"time"

	dbmodel "github.com/evergreen-ci/cedar/model"
	"github.com/stretchr/testify/assert"
)

func TestBuildloggerImport(t *testing.T) {
	t.Run("InvalidType", func(t *testing.T) {
		apiLog := &APILog{}
		assert.Error(t, apiLog.Import(dbmodel.PerformanceResult{}))
	})
	t.Run("ValidLog", func(t *testing.T) {
		log := dbmodel.Log{
			Info: dbmodel.LogInfo{
				Project:     "project",
				Version:     "version",
				Variant:     "variant",
				TaskName:    "task_name",
				TaskID:      "task_id",
				Execution:   2,
				TestName:    "test_name",
				Trial:       3,
				ProcessName: "proc",
				Format:      dbmodel.LogFormatJSON,
				Arguments:   map[string]string{"arg1": "arg", "arg2": "arg"},
				ExitCode:    2,
				Mainline:    true,
				Schema:      0,
			},
			CreatedAt:   time.Now().Add(-1 * time.Hour),
			CompletedAt: time.Now(),
			Artifact: dbmodel.LogArtifactInfo{
				Type:    dbmodel.PailS3,
				Prefix:  "pre",
				Version: 2,
				Chunks: []dbmodel.LogChunkInfo{
					{
						Key:      "key1",
						NumLines: 200,
						Start:    time.Now().Add(-24 * time.Hour),
						End:      time.Now().Add(-23 * time.Hour),
					},
					{
						Key:      "key2",
						NumLines: 101,
						Start:    time.Now().Add(-23 * time.Hour),
						End:      time.Now().Add(-22 * time.Hour),
					},
				},
			},
		}
		log.ID = log.Info.ID()
		expected := &APILog{
			ID: ToAPIString(log.ID),
			Info: APILogInfo{
				Project:     ToAPIString(log.Info.Project),
				Version:     ToAPIString(log.Info.Version),
				Variant:     ToAPIString(log.Info.Variant),
				TaskName:    ToAPIString(log.Info.TaskName),
				TaskID:      ToAPIString(log.Info.TaskID),
				Execution:   log.Info.Execution,
				TestName:    ToAPIString(log.Info.TestName),
				Trial:       3,
				ProcessName: ToAPIString(log.Info.ProcessName),
				Format:      ToAPIString(string(log.Info.Format)),
				Arguments:   log.Info.Arguments,
				ExitCode:    2,
			},
			CreatedAt:   NewTime(log.CreatedAt),
			CompletedAt: NewTime(log.CompletedAt),
			Duration:    float64(log.CompletedAt.Sub(log.CreatedAt)) / float64(time.Second),
			Artifact: APILogArtifactInfo{
				Type:    ToAPIString(string(log.Artifact.Type)),
				Prefix:  ToAPIString(log.Artifact.Prefix),
				Version: 2,
				Chunks: []APILogChunkInfo{
					{
						Key:      ToAPIString(log.Artifact.Chunks[0].Key),
						NumLines: 200,
						Start:    NewTime(log.Artifact.Chunks[0].Start),
						End:      NewTime(log.Artifact.Chunks[0].End),
					},
					{
						Key:      ToAPIString(log.Artifact.Chunks[1].Key),
						NumLines: 101,
						Start:    NewTime(log.Artifact.Chunks[1].Start),
						End:      NewTime(log.Artifact.Chunks[1].End),
					},
				},
			},
		}

		apiLog := &APILog{}
		assert.NoError(t, apiLog.Import(log))
		assert.Equal(t, expected, apiLog)
	})
}
