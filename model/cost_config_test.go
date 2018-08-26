package model

import (
	"context"
	"testing"

	"github.com/evergreen-ci/sink"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCostConfig(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env := sink.GetEnvironment()

	cleanup := func() {
		require.NoError(t, env.Configure(&sink.Configuration{
			MongoDBURI:    "mongodb://localhost:27017",
			DatabaseName:  "sink_test_costconfig",
			NumWorkers:    2,
			UseLocalQueue: true,
		}))

		conf, session, err := sink.GetSessionWithConfig(env)
		require.NoError(t, err)
		if err := session.DB(conf.DatabaseName).DropDatabase(); err != nil {
			assert.Contains(t, err.Error(), "not found")
		}
	}

	defer cleanup()

	for name, test := range map[string]func(context.Context, *testing.T, sink.Environment, *CostConfig){
		"VerifyFixtures": func(ctx context.Context, t *testing.T, env sink.Environment, conf *CostConfig) {
			assert.NotNil(t, env)
			assert.NotNil(t, conf)
			assert.True(t, conf.IsNil())
		},
		"FindErrorsWithoutConfig": func(ctx context.Context, t *testing.T, env sink.Environment, conf *CostConfig) {
			assert.Error(t, conf.Find())
		},
		"FindErrorsWithNoResults": func(ctx context.Context, t *testing.T, env sink.Environment, conf *CostConfig) {
			conf.Setup(env)
			err := conf.Find()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "could not find")
		},
		"FindErrorsWthBadDbName": func(ctx context.Context, t *testing.T, env sink.Environment, conf *CostConfig) {
			require.NoError(t, env.Configure(&sink.Configuration{
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
		"SimpleRoundTrip": func(ctx context.Context, t *testing.T, env sink.Environment, conf *CostConfig) {
			t.Skip("need valid mock")
			conf.Opts.Directory = "foo"
			conf.populated = true
			conf.Setup(env)
			assert.NoError(t, conf.Save())
			err := conf.Find()
			assert.NoError(t, err)
		},
		"SaveErrorsWithBadDBName": func(ctx context.Context, t *testing.T, env sink.Environment, conf *CostConfig) {
			t.Skip("need valid mock")
			require.NoError(t, env.Configure(&sink.Configuration{
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
		"SaveErrorsWithNoEnvConfigured": func(ctx context.Context, t *testing.T, env sink.Environment, conf *CostConfig) {
			t.Skip("need valid mock")
			conf.populated = true
			err := conf.Save()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "env is nil")
		},

		// "": func(ctx context.Context, t *testing.T, env sink.Environment, conf *CostConfig) {},
		// "": func(ctx context.Context, t *testing.T, env sink.Environment, conf *CostConfig) {},
		// "": func(ctx context.Context, t *testing.T, env sink.Environment, conf *CostConfig) {},
		// "": func(ctx context.Context, t *testing.T, env sink.Environment, conf *CostConfig) {},
		// "": func(ctx context.Context, t *testing.T, env sink.Environment, conf *CostConfig) {},
		// "": func(ctx context.Context, t *testing.T, env sink.Environment, conf *CostConfig) {},
	} {
		t.Run(name, func(t *testing.T) {
			cleanup()
			tctx, cancel := context.WithCancel(ctx)
			defer cancel()
			test(tctx, t, env, &CostConfig{})
		})
	}
}
