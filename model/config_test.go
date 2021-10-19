package model

import (
	"context"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/utility"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testDBName = "cedar_test_config"

func init() {
	env, err := cedar.NewEnvironment(context.Background(), testDBName, &cedar.Configuration{
		MongoDBURI:            "mongodb://localhost:27017",
		DatabaseName:          testDBName,
		SocketTimeout:         time.Minute,
		NumWorkers:            2,
		DisableRemoteQueue:    true,
		DisableDBValueCaching: true,
	})
	if err != nil {
		panic(err)
	}

	cedar.SetEnvironment(env)
}

func TestCedarConfig(t *testing.T) {
	conf, session, err := cedar.GetSessionWithConfig(cedar.GetEnvironment())
	require.NoError(t, err)
	require.NoError(t, session.DB(conf.DatabaseName).DropDatabase())

	for name, test := range map[string]func(context.Context, *testing.T, cedar.Environment, *CedarConfig){
		"VerifyFixtures": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *CedarConfig) {
			assert.NotNil(t, env)
			assert.NotNil(t, conf)
			assert.True(t, conf.IsNil())
		},
		"FindErrorsWithoutConfig": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *CedarConfig) {
			assert.Error(t, conf.Find())
		},
		"FindErrorsWithNoResults": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *CedarConfig) {
			conf.Setup(env)
			err := conf.Find()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "could not find")
		},
		"FindErrorsWthBadDbName": func(ctx context.Context, t *testing.T, _ cedar.Environment, conf *CedarConfig) {
			env, err := cedar.NewEnvironment(ctx, "broken-db-test", &cedar.Configuration{
				MongoDBURI:              "mongodb://localhost:27017",
				DatabaseName:            "\"", // intentionally invalid
				NumWorkers:              2,
				DisableRemoteQueue:      true,
				DisableRemoteQueueGroup: true,
			})
			require.NoError(t, err)

			conf.Setup(env)
			err = conf.Find()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "finding")
		},
		"SimpleRoundTrip": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *CedarConfig) {
			conf.Slack.Token = "foo"
			conf.populated = true
			conf.Setup(env)
			assert.NoError(t, conf.Save())
			err := conf.Find()
			assert.NoError(t, err)
		},
		"SaveErrorsWithBadDBName": func(ctx context.Context, t *testing.T, _ cedar.Environment, conf *CedarConfig) {
			env, err := cedar.NewEnvironment(ctx, "broken-db-test", &cedar.Configuration{
				MongoDBURI:              "mongodb://localhost:27017",
				DatabaseName:            "\"", // intentionally invalid
				NumWorkers:              2,
				DisableRemoteQueue:      true,
				DisableRemoteQueueGroup: true,
			})
			require.NoError(t, err)

			conf.Setup(env)
			conf.populated = true
			err = conf.Save()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "saving application")
		},
		"SaveErrorsWithNoEnvConfigured": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *CedarConfig) {
			conf.populated = true
			err := conf.Save()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "environment")
		},
		"ConfFlagsErrorsWithNoEnv": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *CedarConfig) {
			flags := &OperationalFlags{}
			assert.Error(t, flags.update("foo", false))
		},
		"FlagsWithEnvRoundTrip": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *CedarConfig) {
			conf.ID = cedarConfigurationID
			conf.env = env
			conf.Flags = OperationalFlags{env: env}
			conf.populated = true
			require.NoError(t, conf.Save())
			assert.NoError(t, conf.Flags.update("foo", true))
			assert.False(t, conf.Flags.DisableInternalMetricsReporting)

			assert.NoError(t, conf.Flags.SetDisableInternalMetricsReporting(true))
			assert.True(t, conf.Flags.DisableInternalMetricsReporting)
		},
		"SetFlagWithBadConfiguration": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *CedarConfig) {
			assert.Error(t, conf.Flags.SetDisableInternalMetricsReporting(true))
			assert.False(t, conf.Flags.DisableInternalMetricsReporting)
		},
		// "": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *CedarConfig) {},
	} {
		t.Run(name, func(t *testing.T) {
			env := cedar.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()

			require.NoError(t, session.DB(conf.DatabaseName).DropDatabase())
			test(ctx, t, env, &CedarConfig{})
		})
	}

	require.NoError(t, session.DB(conf.DatabaseName).DropDatabase())
}

func TestCachedConfig(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	env, err := cedar.NewEnvironment(ctx, "test", &cedar.Configuration{
		MongoDBURI:              "mongodb://localhost:27017",
		NumWorkers:              2,
		DisableRemoteQueue:      true,
		DisableRemoteQueueGroup: true,
		DatabaseName:            "cached-config-test",
	})
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, env.GetDB().Drop(ctx))
	}()

	conf := NewCedarConfig(env)
	conf.URL = "https://cedar.mongodb.com"
	require.NoError(t, conf.Save())

	_, ok := env.GetCachedDBValue(cedarConfigurationID)
	require.False(t, ok)
	conf = NewCedarConfig(env)
	require.NoError(t, conf.Find())
	require.Equal(t, "https://cedar.mongodb.com", conf.URL)
	t.Run("FirstFindSetsEnvCache", func(t *testing.T) {
		value, ok := env.GetCachedDBValue(cedarConfigurationID)
		require.True(t, ok)
		cachedConf, ok := value.(CedarConfig)
		require.True(t, ok)
		assert.Equal(t, conf.URL, cachedConf.URL)
	})

	newConf := NewCedarConfig(env)
	newConf.URL = "https://evergreen.mongodb.com"
	require.NoError(t, newConf.Save())
	t.Run("CacheGetsUpdated", func(t *testing.T) {
		retryOp := func() (bool, error) {
			value, ok := env.GetCachedDBValue(cedarConfigurationID)
			if !ok {
				return true, errors.New("cached config not found")
			}
			cachedConf, ok := value.(CedarConfig)
			if !ok {
				return false, errors.New("invalid cached config type")
			}
			if newConf.URL != cachedConf.URL {
				return true, errors.New("cached config not updated")
			}
			return false, nil
		}
		require.NoError(t, utility.Retry(ctx, retryOp, utility.RetryOptions{MaxAttempts: 5}))

		newConf = NewCedarConfig(env)
		require.NoError(t, newConf.Find())
		require.Equal(t, "https://evergreen.mongodb.com", newConf.URL)
	})
}
