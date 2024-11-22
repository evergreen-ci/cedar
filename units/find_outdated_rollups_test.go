package units

import (
	"context"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/perf"
	"github.com/mongodb/amboy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func TestFindOutdatedRollupsJob(t *testing.T) {
	env := cedar.GetEnvironment()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	defer func() {
		assert.NoError(t, tearDownEnv(env))
	}()

	ftdcArtifact := model.ArtifactInfo{
		Type:   model.PailLocal,
		Bucket: "testdata",
		Path:   "valid.ftdc",
		Format: model.FileFTDC,
		Schema: model.SchemaRawEvents,
	}

	factories := perf.DefaultRollupFactories()
	require.True(t, len(factories) >= 3)
	factories = factories[0:3]
	require.Len(t, factories[0].Names(), 1)
	require.Len(t, factories[1].Names(), 1)
	require.Len(t, factories[2].Names(), 1)

	resultInfo := model.PerformanceResultInfo{Project: "HasOutdatedRollups", Mainline: true}
	outdatedResult := model.CreatePerformanceResult(resultInfo, []model.ArtifactInfo{ftdcArtifact}, nil)
	outdatedResult.CreatedAt = time.Now().Add(-90*24*time.Hour + time.Hour)
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
	assert.NoError(t, outdatedResult.SaveNew(ctx))

	resultInfo = model.PerformanceResultInfo{Project: "HasOutdatedRollupsButOld"}
	oldOutdatedResult := model.CreatePerformanceResult(resultInfo, []model.ArtifactInfo{ftdcArtifact}, nil)
	oldOutdatedResult.CreatedAt = time.Now().Add(-90 * 24 * time.Hour)
	oldOutdatedResult.Rollups.Stats = outdatedResult.Rollups.Stats
	oldOutdatedResult.Setup(env)
	assert.NoError(t, oldOutdatedResult.SaveNew(ctx))

	resultInfo = model.PerformanceResultInfo{Project: "HasUpdatedRollups"}
	updatedResult := model.CreatePerformanceResult(resultInfo, []model.ArtifactInfo{ftdcArtifact}, nil)
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
	assert.NoError(t, updatedResult.SaveNew(ctx))

	resultInfo = model.PerformanceResultInfo{Project: "TooManyFailures"}
	tooManyFailuresResult := model.CreatePerformanceResult(resultInfo, []model.ArtifactInfo{ftdcArtifact}, nil)
	tooManyFailuresResult.CreatedAt = time.Now()
	tooManyFailuresResult.FailedRollupAttempts = 4
	tooManyFailuresResult.Setup(env)
	assert.NoError(t, tooManyFailuresResult.SaveNew(ctx))

	resultInfo = model.PerformanceResultInfo{Project: "NoFTDCData"}
	noFTDCResult := model.CreatePerformanceResult(resultInfo, nil, nil)
	noFTDCResult.CreatedAt = time.Now()
	noFTDCResult.Setup(env)
	assert.NoError(t, noFTDCResult.SaveNew(ctx))

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

		j.Run(ctx)
		assert.True(t, j.Status().Completed)
		assert.False(t, j.HasErrors())
		amboy.Wait(ctx, env.GetRemoteQueue())

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
		assert.Equal(t, 2, count)
	})
	t.Run("InvalidRollups", func(t *testing.T) {
		j := &findOutdatedRollupsJob{
			RollupTypes: invalidRollupTypes,
			env:         env,
		}
		assert.NoError(t, j.validate())

		j.Run(ctx)
		assert.True(t, j.Status().Completed)
		assert.Error(t, j.Error())
		amboy.Wait(ctx, env.GetRemoteQueue())

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
		j.Run(ctx)
		assert.True(t, j.Status().Completed)
	})
}
