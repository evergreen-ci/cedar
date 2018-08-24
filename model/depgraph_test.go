package model

import (
	"context"
	"testing"

	"github.com/evergreen-ci/sink"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGraphMetadata(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env := sink.GetEnvironment()
	require.NoError(t, env.Configure(&sink.Configuration{
		MongoDBURI:    "mongodb://localhost:27017",
		DatabaseName:  "sink.test.graphmetadata",
		NumWorkers:    2,
		UseLocalQueue: true,
	}))

	defer func() {
		conf, session, err := sink.GetSessionWithConfig(env)
		require.NoError(t, err)
		if err := session.DB(conf.DatabaseName).DropDatabase(); err != nil {
			assert.Contains(t, err.Error(), "not found")
		}
	}()

	for name, test := range map[string]func(context.Context, *testing.T, sink.Environment, *GraphMetadata){
		// "": func(ctx context.Context, t *testing.t, env sink.Environment, conf *Costreport) {},
	} {
		t.Run(name, func(t *testing.T) {
			tctx, cancel := context.WithCancel(ctx)
			defer cancel()
			test(tctx, t, env, &GraphMetadata{})
		})
	}
}
