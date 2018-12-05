package model

import (
	"context"
	"testing"

	"github.com/evergreen-ci/cedar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSinkConfig(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env := cedar.GetEnvironment()

	cleanup := func() {
		require.NoError(t, env.Configure(&cedar.Configuration{
			MongoDBURI:    "mongodb://localhost:27017",
			DatabaseName:  "cedar_test_config",
			NumWorkers:    2,
			UseLocalQueue: true,
		}))

		conf, session, err := cedar.GetSessionWithConfig(env)
		require.NoError(t, err)
		if err := session.DB(conf.DatabaseName).DropDatabase(); err != nil {
			assert.Contains(t, err.Error(), "not found")
		}
	}

	defer cleanup()

	for name, test := range map[string]func(context.Context, *testing.T, cedar.Environment, *SinkConfig){
		"VerifyFixtures": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *SinkConfig) {
			assert.NotNil(t, env)
			assert.NotNil(t, conf)
			assert.True(t, conf.IsNil())
		},
		"FindErrorsWithoutConfig": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *SinkConfig) {
			assert.Error(t, conf.Find())
		},
		"FindErrorsWithNoResults": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *SinkConfig) {
			conf.Setup(env)
			err := conf.Find()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "could not find")
		},
		"FindErrorsWthBadDbName": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *SinkConfig) {
			require.NoError(t, env.Configure(&cedar.Configuration{
				MongoDBURI:    "mongodb://localhost:27017",
				DatabaseName:  "\"", // intentionally invalid
				NumWorkers:    2,
				UseLocalQueue: true,
			}))

			conf.Setup(env)
			err := conf.Find()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "problem finding")
		},
		"SimpleRoundTrip": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *SinkConfig) {
			conf.Slack.Token = "foo"
			conf.populated = true
			conf.Setup(env)
			assert.NoError(t, conf.Save())
			err := conf.Find()
			assert.NoError(t, err)
		},
		"SaveErrorsWithBadDBName": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *SinkConfig) {
			require.NoError(t, env.Configure(&cedar.Configuration{
				MongoDBURI:    "mongodb://localhost:27017",
				DatabaseName:  "\"", // intentionally invalid
				NumWorkers:    2,
				UseLocalQueue: true,
			}))

			conf.Setup(env)
			conf.populated = true
			err := conf.Save()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "problem saving application")
		},
		"SaveErrorsWithNoEnvConfigured": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *SinkConfig) {
			conf.populated = true
			err := conf.Save()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "env is nil")
		},
		// "": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *SinkConfig) {},
		// "": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *SinkConfig) {},
	} {
		t.Run(name, func(t *testing.T) {
			cleanup()
			tctx, cancel := context.WithCancel(ctx)
			defer cancel()
			test(tctx, t, env, &SinkConfig{})
		})
	}
}
