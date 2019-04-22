package units

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/perf"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/mgo.v2/bson"
)

const testDBName = "cedar_test_config"

func init() {
	env, err := cedar.NewEnvironment(context.Background(), testDBName, &cedar.Configuration{
		MongoDBURI:    "mongodb://localhost:27017",
		DatabaseName:  testDBName,
		SocketTimeout: time.Minute,
		NumWorkers:    2,
	})
	if err != nil {
		panic(err)
	}

	cedar.SetEnvironment(env)
}

func tearDownEnv(env cedar.Environment) error {
	conf, session, err := cedar.GetSessionWithConfig(env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()
	return errors.WithStack(session.DB(conf.DatabaseName).DropDatabase())
}

func TestFTDCRollupsJob(t *testing.T) {
	if runtime.GOOS == "darwin" && os.Getenv("EVR_TASK_ID") != "" {
		t.Skip("avoid less relevant failing test in evergreen")
	}

	env := cedar.GetEnvironment()

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
	resultInfo := model.PerformanceResultInfo{Project: "valid"}
	validResult := model.CreatePerformanceResult(resultInfo, []model.ArtifactInfo{validArtifact})
	validResult.Setup(env)
	assert.NoError(t, validResult.Save())
	resultInfo = model.PerformanceResultInfo{Project: "invalid"}
	invalidResult := model.CreatePerformanceResult(resultInfo, []model.ArtifactInfo{invalidArtifact})
	invalidResult.Setup(env)
	assert.NoError(t, invalidResult.Save())

	validRollupTypes := []string{}
	for _, factory := range perf.DefaultRollupFactories() {
		validRollupTypes = append(validRollupTypes, factory.Type())
	}
	invalidRollupTypes := append([]string{"DNE"}, validRollupTypes...)

	t.Run("ValidData", func(t *testing.T) {
		for _, user := range []bool{true, false} {
			j := &ftdcRollupsJob{
				PerfID:        validResult.ID,
				ArtifactInfo:  &validArtifact,
				RollupTypes:   validRollupTypes,
				UserSubmitted: user,
			}
			assert.NoError(t, j.validate())
			ctx, cancel := env.Context()
			defer cancel()

			j.Run(ctx)
			assert.True(t, j.Status().Completed)
			assert.False(t, j.HasErrors())
			result := &model.PerformanceResult{}
			res := env.GetDB().Collection("perf_results").FindOne(ctx, bson.M{"_id": validResult.ID})
			require.NoError(t, res.Err())
			assert.NoError(t, res.Decode(result))
			require.NotNil(t, result.Rollups)
			assert.True(t, len(j.RollupTypes) <= len(result.Rollups.Stats))
			for _, stats := range result.Rollups.Stats {
				assert.Equal(t, user, stats.UserSubmitted)
			}
		}
	})
	t.Run("InvalidRollupTypes", func(t *testing.T) {
		j := &ftdcRollupsJob{
			PerfID:       validResult.ID,
			ArtifactInfo: &validArtifact,
			RollupTypes:  invalidRollupTypes,
		}
		assert.NoError(t, j.validate())
		ctx, cancel := env.Context()
		defer cancel()

		j.Run(ctx)
		assert.True(t, j.Status().Completed)
		assert.Equal(t, 1, j.ErrorCount())
		result := &model.PerformanceResult{}
		res := env.GetDB().Collection("perf_results").FindOne(ctx, bson.M{"_id": validResult.ID})
		require.NoError(t, res.Err())
		assert.NoError(t, res.Decode(result))
		require.NotNil(t, result.Rollups)
		assert.True(t, len(j.RollupTypes)-1 <= len(result.Rollups.Stats))
	})
	t.Run("InvalidData", func(t *testing.T) {
		j := &ftdcRollupsJob{
			PerfID:       invalidResult.ID,
			ArtifactInfo: &invalidArtifact,
			RollupTypes:  validRollupTypes,
		}
		assert.NoError(t, j.validate())
		j.Run(context.TODO())
		assert.True(t, j.Status().Completed)
		assert.Equal(t, 1, j.ErrorCount())
	})
	t.Run("InvalidID", func(t *testing.T) {
		j := &ftdcRollupsJob{
			PerfID:       "DNE",
			ArtifactInfo: &validArtifact,
			RollupTypes:  validRollupTypes,
		}
		assert.NoError(t, j.validate())
		j.Run(context.TODO())
		assert.True(t, j.Status().Completed)
		assert.Equal(t, 1, j.ErrorCount())
	})
	t.Run("InvalidSetup", func(t *testing.T) {
		j := &ftdcRollupsJob{
			PerfID:      validResult.ID,
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
			PerfID:       validResult.ID,
			ArtifactInfo: &validArtifact,
		}
		assert.Error(t, j.validate())
		assert.False(t, j.Status().Completed)

	})
	t.Run("InvalidBucket", func(t *testing.T) {
		j := &ftdcRollupsJob{
			PerfID: validResult.ID,
			ArtifactInfo: &model.ArtifactInfo{
				Type:   model.PailLocal,
				Bucket: "DNE",
				Path:   "valid.ftdc",
			},
			RollupTypes: validRollupTypes,
		}
		assert.NoError(t, j.validate())
		j.Run(context.TODO())
		assert.True(t, j.Status().Completed)
		assert.Equal(t, 1, j.ErrorCount())
	})
	t.Run("PublicFunction", func(t *testing.T) {
		for _, user := range []bool{true, false} {
			j, err := NewFTDCRollupsJob(validResult.ID, &validArtifact, perf.DefaultRollupFactories(), user)
			require.NoError(t, err)
			assert.Equal(t, validResult.ID, j.(*ftdcRollupsJob).PerfID)
			assert.Equal(t, &validArtifact, j.(*ftdcRollupsJob).ArtifactInfo)
			assert.Equal(t, user, j.(*ftdcRollupsJob).UserSubmitted)
			j.Run(context.TODO())
			assert.True(t, j.Status().Completed)
		}
	})
}
