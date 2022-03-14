package units

import (
	"context"
	"os"
	"runtime"
	"testing"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/perf"
	"github.com/mongodb/amboy/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func TestFTDCRollupsJob(t *testing.T) {
	if runtime.GOOS == "darwin" && os.Getenv("EVR_TASK_ID") != "" {
		t.Skip("avoid less relevant failing test in evergreen")
	}

	env := cedar.GetEnvironment()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	defer func() {
		assert.NoError(t, tearDownEnv(env))
	}()

	validArtifact := model.ArtifactInfo{
		Type:   model.PailLocal,
		Bucket: "testdata",
		Path:   "valid.ftdc",
	}
	invalidArtifact := model.ArtifactInfo{
		Type:   model.PailLocal,
		Bucket: "testdata",
		Path:   "invalid.ftdc",
	}
	resultInfo := model.PerformanceResultInfo{Project: "valid_mainline", Mainline: true}
	validResultMainline := model.CreatePerformanceResult(resultInfo, []model.ArtifactInfo{validArtifact}, nil)
	validResultMainline.Setup(env)
	assert.NoError(t, validResultMainline.SaveNew(ctx))
	resultInfo = model.PerformanceResultInfo{Project: "valid_patch"}
	validResultPatch := model.CreatePerformanceResult(resultInfo, []model.ArtifactInfo{validArtifact}, nil)
	validResultPatch.Setup(env)
	assert.NoError(t, validResultPatch.SaveNew(ctx))
	resultInfo = model.PerformanceResultInfo{Project: "invalid"}
	invalidResult := model.CreatePerformanceResult(resultInfo, []model.ArtifactInfo{invalidArtifact}, nil)
	invalidResult.Setup(env)
	assert.NoError(t, invalidResult.SaveNew(ctx))

	validRollupTypes := []string{}
	for _, factory := range perf.DefaultRollupFactories() {
		validRollupTypes = append(validRollupTypes, factory.Type())
	}
	invalidRollupTypes := append([]string{"DNE"}, validRollupTypes...)

	t.Run("ValidDataMainline", func(t *testing.T) {
		for _, user := range []bool{true, false} {
			j := &ftdcRollupsJob{
				PerfID:        validResultMainline.ID,
				ArtifactInfo:  &validArtifact,
				RollupTypes:   validRollupTypes,
				UserSubmitted: user,
			}
			assert.NoError(t, j.validate())

			j.queue = queue.NewLocalLimitedSize(1, 100)
			_ = j.queue.Start(ctx)
			j.Run(ctx)
			assert.True(t, j.Status().Completed)
			assert.False(t, j.HasErrors())
			result := &model.PerformanceResult{}
			res := env.GetDB().Collection("perf_results").FindOne(ctx, bson.M{"_id": validResultMainline.ID})
			require.NoError(t, res.Err())
			assert.NoError(t, res.Decode(result))
			require.NotNil(t, result.Rollups)
			assert.True(t, len(j.RollupTypes) <= len(result.Rollups.Stats))
			assert.Equal(t, 1, j.queue.Stats(ctx).Total)
			assert.Zero(t, result.FailedRollupAttempts)
			for _, stats := range result.Rollups.Stats {
				assert.Equal(t, user, stats.UserSubmitted)
			}
		}
	})
	t.Run("ValidDataPatch", func(t *testing.T) {
		for _, user := range []bool{true, false} {
			j := &ftdcRollupsJob{
				PerfID:        validResultPatch.ID,
				ArtifactInfo:  &validArtifact,
				RollupTypes:   validRollupTypes,
				UserSubmitted: user,
			}
			assert.NoError(t, j.validate())

			j.queue = queue.NewLocalLimitedSize(1, 100)
			_ = j.queue.Start(ctx)
			j.Run(ctx)
			assert.True(t, j.Status().Completed)
			assert.False(t, j.HasErrors())
			result := &model.PerformanceResult{}
			res := env.GetDB().Collection("perf_results").FindOne(ctx, bson.M{"_id": validResultPatch.ID})
			require.NoError(t, res.Err())
			assert.NoError(t, res.Decode(result))
			require.NotNil(t, result.Rollups)
			assert.True(t, len(j.RollupTypes) <= len(result.Rollups.Stats))
			assert.Equal(t, 0, j.queue.Stats(ctx).Total)
			assert.Zero(t, result.FailedRollupAttempts)
			for _, stats := range result.Rollups.Stats {
				assert.Equal(t, user, stats.UserSubmitted)
			}
		}
	})
	t.Run("InvalidRollupTypes", func(t *testing.T) {
		j := &ftdcRollupsJob{
			PerfID:       validResultMainline.ID,
			ArtifactInfo: &validArtifact,
			RollupTypes:  invalidRollupTypes,
		}
		assert.NoError(t, j.validate())

		j.Run(ctx)
		assert.True(t, j.Status().Completed)
		assert.Error(t, j.Error())
		result := &model.PerformanceResult{}
		res := env.GetDB().Collection("perf_results").FindOne(ctx, bson.M{"_id": validResultMainline.ID})
		require.NoError(t, res.Err())
		assert.NoError(t, res.Decode(result))
		require.NotNil(t, result.Rollups)
		assert.Zero(t, result.FailedRollupAttempts)
		assert.True(t, len(j.RollupTypes)-1 <= len(result.Rollups.Stats))
	})
	t.Run("InvalidData", func(t *testing.T) {
		j := &ftdcRollupsJob{
			PerfID:       invalidResult.ID,
			ArtifactInfo: &invalidArtifact,
			RollupTypes:  validRollupTypes,
		}
		assert.NoError(t, j.validate())
		j.Run(ctx)
		assert.True(t, j.Status().Completed)
		assert.Error(t, j.Error())

		result := &model.PerformanceResult{}
		res := env.GetDB().Collection("perf_results").FindOne(ctx, bson.M{"_id": invalidResult.ID})
		require.NoError(t, res.Err())
		assert.NoError(t, res.Decode(result))
		assert.Equal(t, 1, result.FailedRollupAttempts)
	})
	t.Run("InvalidID", func(t *testing.T) {
		j := &ftdcRollupsJob{
			PerfID:       "DNE",
			ArtifactInfo: &validArtifact,
			RollupTypes:  validRollupTypes,
		}
		assert.NoError(t, j.validate())
		j.Run(ctx)
		assert.True(t, j.Status().Completed)
		assert.Error(t, j.Error())
	})
	t.Run("InvalidSetup", func(t *testing.T) {
		j := &ftdcRollupsJob{
			PerfID:      validResultMainline.ID,
			RollupTypes: validRollupTypes,
		}
		assert.Error(t, j.validate())
		assert.False(t, j.Status().Completed)

		j = &ftdcRollupsJob{
			ArtifactInfo: &validArtifact,
			RollupTypes:  validRollupTypes,
		}
		assert.Error(t, j.validate())
		assert.False(t, j.Status().Completed)

		j = &ftdcRollupsJob{
			PerfID:       validResultMainline.ID,
			ArtifactInfo: &validArtifact,
		}
		assert.Error(t, j.validate())
		assert.False(t, j.Status().Completed)

	})
	t.Run("InvalidBucket", func(t *testing.T) {
		j := &ftdcRollupsJob{
			PerfID: validResultMainline.ID,
			ArtifactInfo: &model.ArtifactInfo{
				Type:   model.PailLocal,
				Bucket: "DNE",
				Path:   "valid.ftdc",
			},
			RollupTypes: validRollupTypes,
		}
		assert.NoError(t, j.validate())
		j.Run(ctx)
		assert.True(t, j.Status().Completed)
		assert.Error(t, j.Error())

		result := &model.PerformanceResult{}
		res := env.GetDB().Collection("perf_results").FindOne(ctx, bson.M{"_id": validResultMainline.ID})
		require.NoError(t, res.Err())
		assert.NoError(t, res.Decode(result))
		assert.Equal(t, 1, result.FailedRollupAttempts)
	})
	t.Run("PublicFunction", func(t *testing.T) {
		for _, user := range []bool{true, false} {
			j, err := NewFTDCRollupsJob(validResultMainline.ID, &validArtifact, perf.DefaultRollupFactories(), user)
			require.NoError(t, err)
			assert.Equal(t, validResultMainline.ID, j.(*ftdcRollupsJob).PerfID)
			assert.Equal(t, &validArtifact, j.(*ftdcRollupsJob).ArtifactInfo)
			assert.Equal(t, user, j.(*ftdcRollupsJob).UserSubmitted)
			j.Run(ctx)
			assert.True(t, j.Status().Completed)
		}
	})
}
