package units

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/perf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/mgo.v2/bson"
)

func TestFindOutdatedRollupsJob(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows is slow")
	}

	env := cedar.GetEnvironment()

	defer func() {
		assert.NoError(t, tearDownEnv(env))
	}()

	ftdcArtifact := model.ArtifactInfo{
		Type:   model.PailLocal,
		Bucket: "testdata",
		Path:   "valid.ftdc",
		Format: model.FileFTDC,
	}

	factories := perf.DefaultRollupFactories()
	require.True(t, len(factories) >= 3)
	factories = factories[0:3]
	require.Len(t, factories[0].Names(), 1)
	require.Len(t, factories[1].Names(), 1)
	require.Len(t, factories[2].Names(), 1)

	resultInfo := model.PerformanceResultInfo{Project: "HasOutdatedRollups"}
	outdatedResult := model.CreatePerformanceResult(resultInfo, []model.ArtifactInfo{ftdcArtifact})
	outdatedResult.CreatedAt = time.Now().Add(-3*24*time.Hour + time.Hour)
	outdatedResult.Rollups.Stats = append(
		outdatedResult.Rollups.Stats,
		model.PerfRollupValue{
			Name:    factories[0].Names()[0],
			Version: factories[0].Version() - 1,
		},
		model.PerfRollupValue{
			Name:    factories[1].Names()[0],
			Version: factories[1].Version() - 1,
		},
	)
	outdatedResult.Setup(env)
	assert.NoError(t, outdatedResult.Save())

	resultInfo = model.PerformanceResultInfo{Project: "HasOutdatedRollupsButOld"}
	oldOutdatedResult := model.CreatePerformanceResult(resultInfo, []model.ArtifactInfo{ftdcArtifact})
	oldOutdatedResult.CreatedAt = time.Now().Add(-3 * 24 * time.Hour)
	oldOutdatedResult.Rollups.Stats = outdatedResult.Rollups.Stats
	oldOutdatedResult.Setup(env)
	assert.NoError(t, oldOutdatedResult.Save())

	resultInfo = model.PerformanceResultInfo{Project: "HasUpdatedRollups"}
	updatedResult := model.CreatePerformanceResult(resultInfo, []model.ArtifactInfo{ftdcArtifact})
	updatedResult.CreatedAt = time.Now()
	updatedResult.Rollups.Stats = append(
		updatedResult.Rollups.Stats,
		model.PerfRollupValue{
			Name:    factories[0].Names()[0],
			Version: factories[0].Version(),
		},
		model.PerfRollupValue{
			Name:    factories[1].Names()[0],
			Version: factories[1].Version(),
		},
		model.PerfRollupValue{
			Name:    factories[2].Names()[0],
			Version: factories[2].Version(),
		},
	)
	updatedResult.Setup(env)
	assert.NoError(t, updatedResult.Save())

	resultInfo = model.PerformanceResultInfo{Project: "NoFTDCData"}
	noFTDCResult := model.CreatePerformanceResult(resultInfo, []model.ArtifactInfo{})
	noFTDCResult.CreatedAt = time.Now()
	noFTDCResult.Setup(env)
	assert.NoError(t, noFTDCResult.Save())

	validRollupTypes := []string{}
	for _, factory := range factories {
		validRollupTypes = append(validRollupTypes, factory.Type())
	}
	invalidRollupTypes := append([]string{"DNE"}, validRollupTypes...)

	t.Run("FindsAndUpdatesCorrectly", func(t *testing.T) {
		j := &findOutdatedRollupsJob{
			RollupTypes: validRollupTypes,
			env:         env,
		}
		assert.NoError(t, j.validate())
		ctx, cancel := env.Context()
		defer cancel()

		j.Run(ctx)
		assert.True(t, j.Status().Completed)
		assert.False(t, j.HasErrors())
		time.Sleep(time.Second)

		result := &model.PerformanceResult{}
		res := env.GetDB().Collection("perf_results").FindOne(ctx, bson.M{"_id": outdatedResult.Info.ID()})
		require.NoError(t, res.Err())
		assert.NoError(t, res.Decode(result))
		require.NotNil(t, result.Rollups)
		require.Equal(t, len(factories), len(result.Rollups.Stats))
		for i := range factories {
			assert.Equal(t, factories[i].Names()[0], result.Rollups.Stats[i].Name)
			assert.Equal(t, factories[i].Version(), result.Rollups.Stats[i].Version)
		}

		result = &model.PerformanceResult{}
		res = env.GetDB().Collection("perf_results").FindOne(ctx, bson.M{"_id": updatedResult.Info.ID()})
		require.NoError(t, res.Err())
		assert.NoError(t, res.Decode(result))
		assert.Equal(t, updatedResult.Rollups.Stats, result.Rollups.Stats)

		result = &model.PerformanceResult{}
		res = env.GetDB().Collection("perf_results").FindOne(ctx, bson.M{"_id": noFTDCResult.Info.ID()})
		require.NoError(t, res.Err())
		assert.NoError(t, res.Decode(result))
		assert.Empty(t, result.Rollups.Stats)

		cursor, err := env.GetDB().Collection("cedar.service.jobs").Find(ctx, bson.M{})
		require.NoError(t, err)
		defer cursor.Close(ctx)
		count := 0
		for cursor.Next(ctx) {
			count += 1
		}
		assert.Equal(t, 1, count)
	})
	t.Run("InvalidRollups", func(t *testing.T) {
		j := &findOutdatedRollupsJob{
			RollupTypes: invalidRollupTypes,
			env:         env,
		}
		assert.NoError(t, j.validate())
		ctx, cancel := env.Context()
		defer cancel()

		j.Run(ctx)
		assert.True(t, j.Status().Completed)
		assert.Equal(t, 1, j.ErrorCount())
		time.Sleep(time.Second)

		result := &model.PerformanceResult{}
		res := env.GetDB().Collection("perf_results").FindOne(ctx, bson.M{"_id": outdatedResult.Info.ID()})
		require.NoError(t, res.Err())
		assert.NoError(t, res.Decode(result))
		require.NotNil(t, result.Rollups)
		require.Equal(t, len(factories), len(result.Rollups.Stats))
		for i := range factories {
			assert.Equal(t, factories[i].Names()[0], result.Rollups.Stats[i].Name)
			assert.Equal(t, factories[i].Version(), result.Rollups.Stats[i].Version)
		}

		result = &model.PerformanceResult{}
		res = env.GetDB().Collection("perf_results").FindOne(ctx, bson.M{"_id": updatedResult.Info.ID()})
		require.NoError(t, res.Err())
		assert.NoError(t, res.Decode(result))
		assert.Equal(t, updatedResult.Rollups.Stats, result.Rollups.Stats)

		result = &model.PerformanceResult{}
		res = env.GetDB().Collection("perf_results").FindOne(ctx, bson.M{"_id": noFTDCResult.Info.ID()})
		require.NoError(t, res.Err())
		assert.NoError(t, res.Decode(result))
		assert.Empty(t, result.Rollups.Stats)
	})
	t.Run("InvalidSetup", func(t *testing.T) {
		j := &findOutdatedRollupsJob{
			env: env,
		}
		assert.Error(t, j.validate())
	})
	t.Run("PublicFunction", func(t *testing.T) {
		j, err := NewFindOutdatedRollupsJob(factories)
		require.NoError(t, err)
		rollupTypes := j.(*findOutdatedRollupsJob).RollupTypes
		for i := range rollupTypes {
			assert.Equal(t, factories[i].Type(), rollupTypes[i])
		}
		j.Run(context.TODO())
		assert.True(t, j.Status().Completed)
	})
}
