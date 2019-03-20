package model

import (
	"context"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (c *CostConfig) PopulateMock() {
	c.Amazon.EBSPrices.GP2 = 1.0
	c.Amazon.S3Info.Bucket = "foo"
	c.Amazon.S3Info.KeyStart = "bar"
	c.Evergreen.RootURL = "http://evergreen.example.net"
	c.Evergreen.User = "user"
	c.Evergreen.Key = "key"
}

func TestCostConfig(t *testing.T) {
	env := cedar.GetEnvironment()
	ctx, cancel := env.Context()
	defer cancel()

	cleanup := func() {
		conf, session, err := cedar.GetSessionWithConfig(env)
		require.NoError(t, err)
		if err := session.DB(conf.DatabaseName).DropDatabase(); err != nil {
			assert.Contains(t, err.Error(), "not found")
		}
	}

	defer cleanup()

	for name, test := range map[string]func(context.Context, *testing.T, cedar.Environment, *CostConfig){
		"VerifyFixtures": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *CostConfig) {
			assert.NotNil(t, env)
			assert.NotNil(t, conf)
			assert.True(t, conf.IsNil())
		},
		"ValidMock": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *CostConfig) {
			conf.PopulateMock()
			assert.NoError(t, conf.Validate())
		},
		"FindErrorsWithoutConfig": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *CostConfig) {
			assert.Error(t, conf.Find())
		},
		"FindErrorsWithNoResults": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *CostConfig) {
			conf.Setup(env)
			err := conf.Find()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "could not find")
		},
		"FindErrorsWthBadDbName": func(ctx context.Context, t *testing.T, _ cedar.Environment, conf *CostConfig) {
			env, err := cedar.NewEnvironment(ctx, "broken", &cedar.Configuration{
				MongoDBURI:         "mongodb://localhost:27017",
				DatabaseName:       "\"", // intentionally invalid
				NumWorkers:         2,
				DisableRemoteQueue: true,
			})
			require.NoError(t, err)

			conf.Setup(env)
			err = conf.Find()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "problem finding")
		},
		"SimpleRoundTrip": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *CostConfig) {
			conf.PopulateMock()
			conf.Opts.Directory = "foo"
			conf.populated = true
			conf.Setup(env)
			assert.NoError(t, conf.Save())
			err := conf.Find()
			assert.NoError(t, err)
		},
		"SaveErrorsWithBadDBName": func(ctx context.Context, t *testing.T, _ cedar.Environment, conf *CostConfig) {
			conf.PopulateMock()
			env, err := cedar.NewEnvironment(ctx, "broken", &cedar.Configuration{
				MongoDBURI:         "mongodb://localhost:27017",
				DatabaseName:       "\"", // intentionally invalid
				NumWorkers:         2,
				DisableRemoteQueue: true,
			})
			require.NoError(t, err)

			conf.Setup(env)
			conf.populated = true
			err = conf.Save()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "problem saving cost reporting configuration")
		},
		"SaveErrorsWithNoEnvConfigured": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *CostConfig) {
			conf.PopulateMock()
			conf.populated = true
			err := conf.Save()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "env is nil")
		},
		"SaveErrorIfPopulatedIsNotTrue": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *CostConfig) {
			conf.PopulateMock()
			assert.True(t, conf.IsNil())
			err := conf.Save()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "cannot save non-populated")
		},
		"DurationMethodMinimumIsAnHour": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *CostConfig) {
			for _, dur := range []time.Duration{time.Second, time.Minute - 2, time.Second * 20} {
				out, err := conf.GetDuration(dur)
				assert.NoError(t, err)
				assert.Equal(t, time.Hour, out)
			}
		},
		"ParseDurationFromStringValid": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *CostConfig) {
			conf.Opts.Duration = time.Hour.String()
			out, err := conf.GetDuration(time.Hour * 24)
			assert.NoError(t, err)
			assert.Equal(t, time.Hour, out)
		},
		"ParseDurationFromStringInValid": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *CostConfig) {
			conf.Opts.Duration = time.Hour.String() + "-foo"
			out, err := conf.GetDuration(time.Hour * 24)
			assert.Error(t, err)
			assert.Equal(t, time.Duration(0), out)
		},
		"ValidatorForEvergreenConnInfo": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *CostConfig) {
			var info *EvergreenConnectionInfo
			assert.False(t, info.IsValid())
			info = &EvergreenConnectionInfo{}
			assert.False(t, info.IsValid())

			conf.PopulateMock()
			info = &conf.Evergreen
			assert.True(t, info.IsValid())
		},
		"ValidatorForS3Config": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *CostConfig) {
			var amz *CostConfigAmazonS3
			assert.False(t, amz.IsValid())
			amz = &CostConfigAmazonS3{}
			assert.False(t, amz.IsValid())

			conf.PopulateMock()
			amz = &conf.Amazon.S3Info
			assert.True(t, amz.IsValid())
		},
		// "": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *CostConfig) {},
	} {
		t.Run(name, func(t *testing.T) {
			cleanup()
			tctx, cancel := context.WithCancel(ctx)
			defer cancel()
			test(tctx, t, env, &CostConfig{})
		})
	}
}
