package model

import (
	"context"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGraphMetadata(t *testing.T) {
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

	for name, test := range map[string]func(context.Context, *testing.T, cedar.Environment, *GraphMetadata){
		"VerifyFixtures": func(ctx context.Context, t *testing.T, env cedar.Environment, graph *GraphMetadata) {
			assert.NotNil(t, env)
			assert.NotNil(t, graph)
			assert.True(t, graph.IsNil())
		},
		"FindErrorsWithoutConfig": func(ctx context.Context, t *testing.T, env cedar.Environment, graph *GraphMetadata) {
			assert.Error(t, graph.Find())
		},
		"FindErrorsWithNoResults": func(ctx context.Context, t *testing.T, env cedar.Environment, graph *GraphMetadata) {
			graph.Setup(env)
			err := graph.Find()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "could not find")
		},
		"FindErrorsWthBadDbName": func(ctx context.Context, t *testing.T, env cedar.Environment, graph *GraphMetadata) {
			env, err := cedar.NewEnvironment(ctx, testDBName, &cedar.Configuration{
				MongoDBURI:         "mongodb://localhost:27017",
				DatabaseName:       "'$",
				SocketTimeout:      time.Minute,
				NumWorkers:         2,
				DisableRemoteQueue: true,
			})
			require.NoError(t, err)

			graph.Setup(env)
			err = graph.Find()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "problem running graph metadata query")
		},
		"SimpleRoundTrip": func(ctx context.Context, t *testing.T, env cedar.Environment, graph *GraphMetadata) {
			graph.BuildID = "foo"
			graph.populated = true
			graph.Setup(env)
			assert.NoError(t, graph.Save())
			err := graph.Find()
			assert.NoError(t, err)
		},
		"SaveErrorsWithBadDBName": func(ctx context.Context, t *testing.T, env cedar.Environment, graph *GraphMetadata) {
			env, err := cedar.NewEnvironment(ctx, testDBName, &cedar.Configuration{
				MongoDBURI:         "mongodb://localhost:27017",
				DatabaseName:       "'$",
				SocketTimeout:      time.Minute,
				NumWorkers:         2,
				DisableRemoteQueue: true,
			})
			require.NoError(t, err)

			graph.BuildID = "bar"
			graph.Setup(env)
			graph.populated = true
			err = graph.Save()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot do createCollection on namespace with a $ in it")
		},
		"SaveErrorsWithNoEnvConfigured": func(ctx context.Context, t *testing.T, env cedar.Environment, graph *GraphMetadata) {
			graph.BuildID = "baz"
			graph.populated = true
			err := graph.Save()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "env is nil")
		},
		// "": func(ctx context.Context, t *testing.T, env cedar.Environment, graph *GraphMetadata) {},
	} {
		t.Run(name, func(t *testing.T) {
			cleanup()
			tctx, cancel := context.WithCancel(ctx)
			defer cancel()
			test(tctx, t, env, &GraphMetadata{})
		})
	}
}
