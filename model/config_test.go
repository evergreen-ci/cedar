package model

import (
	"context"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testDBName = "cedar_test_config"

func init() {
	env, err := cedar.NewEnvironment(context.Background(), testDBName, &cedar.Configuration{
		MongoDBURI:         "mongodb://localhost:27017",
		DatabaseName:       testDBName,
		SocketTimeout:      time.Minute,
		NumWorkers:         2,
		DisableRemoteQueue: true,
		DBUser:             "myUserAdmin",
		DBPwd:              "default",
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
				DBUser:                  "myUserAdmin",
				DBPwd:                   "default",
			})
			require.NoError(t, err)

			conf.Setup(env)
			err = conf.Find()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "problem finding")
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
				DBUser:                  "myUserAdmin",
				DBPwd:                   "default",
			})
			require.NoError(t, err)

			conf.Setup(env)
			conf.populated = true
			err = conf.Save()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "problem saving application")
		},
		"SaveErrorsWithNoEnvConfigured": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *CedarConfig) {
			conf.populated = true
			err := conf.Save()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "env is nil")
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
			assert.False(t, conf.Flags.DisableCostReportingJob)

			assert.NoError(t, conf.Flags.SetDisableCostReportingJob(true))
			assert.True(t, conf.Flags.DisableCostReportingJob)
		},
		"SetFlagWithBadConfiguration": func(ctx context.Context, t *testing.T, env cedar.Environment, conf *CedarConfig) {
			assert.Error(t, conf.Flags.SetDisableCostReportingJob(true))
			assert.False(t, conf.Flags.DisableCostReportingJob)
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
