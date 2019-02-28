package units

import (
	"context"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	validDataFile   = "testdata/valid.ftdc"
	invalidDataFile = "testdata/invalid.ftdc"
)

func createEnv() (cedar.Environment, error) {
	env := cedar.GetEnvironment()
	err := env.Configure(&cedar.Configuration{
		MongoDBURI:         "mongodb://localhost:27017",
		DatabaseName:       "ftdc_rollups_job_test",
		SocketTimeout:      time.Hour,
		NumWorkers:         2,
		DisableRemoteQueue: true,
	})
	return env, errors.WithStack(err)
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
	env, err := createEnv()
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, tearDownEnv(env))
	}()

	conf, sess, err := cedar.GetSessionWithConfig(env)
	require.NoError(t, err)

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
	validResult.Save()
	resultInfo = model.PerformanceResultInfo{Project: "invalid"}
	invalidResult := model.CreatePerformanceResult(resultInfo, []model.ArtifactInfo{invalidArtifact})
	invalidResult.Setup(env)
	invalidResult.Save()

	t.Run("ValidData", func(t *testing.T) {
		rollupCalculator := &ftdcRollups{
			PerfID:       validResult.ID,
			ArtifactInfo: &validArtifact,
		}
		assert.NoError(t, rollupCalculator.Validate())
		rollupCalculator.Run(context.TODO())
		assert.True(t, rollupCalculator.Status().Completed)
		assert.False(t, rollupCalculator.HasErrors())
		assert.Nil(t, rollupCalculator.ArtifactInfo)
		result := &model.PerformanceResult{}
		assert.NoError(t, sess.DB(conf.DatabaseName).C("perf_results").FindId(validResult.ID).One(result))
		assert.NotEmpty(t, result.Rollups.Stats)
	})
	t.Run("InvalidData", func(t *testing.T) {
		rollupCalculator := &ftdcRollups{
			PerfID:       invalidResult.ID,
			ArtifactInfo: &invalidArtifact,
		}
		assert.NoError(t, rollupCalculator.Validate())
		rollupCalculator.Run(context.TODO())
		assert.True(t, rollupCalculator.Status().Completed)
		assert.Equal(t, 1, rollupCalculator.ErrorCount())
		assert.Nil(t, rollupCalculator.ArtifactInfo)
	})
	t.Run("InvalidID", func(t *testing.T) {
		rollupCalculator := &ftdcRollups{
			PerfID:       "DNE",
			ArtifactInfo: &validArtifact,
		}
		assert.NoError(t, rollupCalculator.Validate())
		rollupCalculator.Run(context.TODO())
		assert.True(t, rollupCalculator.Status().Completed)
		assert.Equal(t, 1, rollupCalculator.ErrorCount())
		assert.Nil(t, rollupCalculator.ArtifactInfo)
	})
	t.Run("InvalidSetup", func(t *testing.T) {
		rollupCalculator := &ftdcRollups{
			PerfID: validResult.ID,
		}
		assert.Error(t, rollupCalculator.Validate())

		rollupCalculator = &ftdcRollups{
			ArtifactInfo: &validArtifact,
		}
		assert.Error(t, rollupCalculator.Validate())
		assert.False(t, rollupCalculator.Status().Completed)
		assert.NotNil(t, rollupCalculator.ArtifactInfo)
	})
	t.Run("InvalidBucket", func(t *testing.T) {
		rollupCalculator := &ftdcRollups{
			PerfID: validResult.ID,
			ArtifactInfo: &model.ArtifactInfo{
				Type:   model.PailLocal,
				Bucket: "DNE",
				Path:   "valid.ftdc",
			},
		}
		assert.NoError(t, rollupCalculator.Validate())
		rollupCalculator.Run(context.TODO())
		assert.True(t, rollupCalculator.Status().Completed)
		assert.Equal(t, 1, rollupCalculator.ErrorCount())
		assert.Nil(t, rollupCalculator.ArtifactInfo)
	})
}
