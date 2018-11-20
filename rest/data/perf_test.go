package data

import (
	"testing"

	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/model"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createEnv() (sink.Environment, error) {
	env := sink.GetEnvironment()
	err := env.Configure(&sink.Configuration{
		MongoDBURI:    "mongodb://localhost:27017",
		DatabaseName:  "grpc_test",
		NumWorkers:    2,
		UseLocalQueue: true,
	})
	return env, errors.WithStack(err)
}

func tearDownEnv(env sink.Environment) error {
	conf, session, err := sink.GetSessionWithConfig(env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()
	return errors.WithStack(session.DB(conf.DatabaseName).DropDatabase())
}

func TestFindPerformanceResultById(t *testing.T) {
	env, err := createEnv()
	require.NoError(t, err)
	defer func() {
		require.NoError(t, tearDownEnv(env))
	}()

	sc := CreateDBConnector(env)

	t.Run("TestWithExistingID", func(t *testing.T) {
		info := model.PerformanceResultInfo{
			Project: "test",
		}
		expectedResult := model.CreatePerformanceResult(info, []model.ArtifactInfo{})
		expectedResult.Setup(env)
		err = expectedResult.Save()
		require.NoError(t, err)

		actualResult, err := sc.FindPerformanceResultById(expectedResult.ID)
		assert.Equal(t, expectedResult.ID, actualResult.ID)
		assert.NoError(t, err)
	})
}
