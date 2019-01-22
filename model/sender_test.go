package model

import (
	"context"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env := cedar.GetEnvironment()
	require.NoError(t, env.Configure(&cedar.Configuration{
		MongoDBURI:    "mongodb://localhost:27017",
		DatabaseName:  "cedar.test.event",
		SocketTimeout: time.Hour,
		NumWorkers:    2,
		UseLocalQueue: true,
	}))

	defer func() {
		conf, session, err := cedar.GetSessionWithConfig(env)
		require.NoError(t, err)
		if err := session.DB(conf.DatabaseName).DropDatabase(); err != nil {
			assert.Contains(t, err.Error(), "not found")
		}
	}()

	for name, test := range map[string]func(context.Context, *testing.T, cedar.Environment, *Event){
		// "": func(ctx context.Context, t *testing.t, env cedar.Environment, conf *Costreport) {},
	} {
		t.Run(name, func(t *testing.T) {
			tctx, cancel := context.WithCancel(ctx)
			defer cancel()
			test(tctx, t, env, &Event{})
		})
	}
}
