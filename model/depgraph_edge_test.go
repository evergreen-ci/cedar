package model

import (
	"context"
	"testing"

	"github.com/evergreen-ci/cedar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGraphEdge(t *testing.T) {
	env := cedar.GetEnvironment()
	ctx, cancel := env.Context()
	defer cancel()

	defer func() {
		conf, session, err := cedar.GetSessionWithConfig(env)
		require.NoError(t, err)
		if err := session.DB(conf.DatabaseName).DropDatabase(); err != nil {
			assert.Contains(t, err.Error(), "not found")
		}
	}()

	for name, test := range map[string]func(context.Context, *testing.T, cedar.Environment, *GraphEdge){
		// "": func(ctx context.Context, t *testing.t, env cedar.Environment, e *GraphEdge) {},
	} {
		t.Run(name, func(t *testing.T) {
			tctx, cancel := context.WithCancel(ctx)
			defer cancel()
			test(tctx, t, env, &GraphEdge{})
		})
	}
}
