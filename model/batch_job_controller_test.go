package model

import (
	"context"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindBatchJobController(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(batchJobControllerCollection).Drop(ctx))
	}()

	c1 := &BatchJobController{
		ID:         "job1",
		Collection: "collection1",
		BatchSize:  1000,
		Iterations: 10,
		Timeout:    10 * time.Minute,
	}
	c2 := &BatchJobController{
		ID:         "job2",
		Collection: "collection2",
		BatchSize:  2000,
		Iterations: 20,
		Timeout:    20 * time.Minute,
	}
	_, err := db.Collection(batchJobControllerCollection).InsertMany(ctx, []interface{}{c1, c2})
	require.NoError(t, err)

	t.Run("DNE", func(t *testing.T) {
		controller, err := FindBatchJobController(ctx, env, "DNE")
		assert.Nil(t, controller)
		assert.Error(t, err)
	})
	t.Run("Exists", func(t *testing.T) {
		controller, err := FindBatchJobController(ctx, env, c1.ID)
		require.NoError(t, err)
		assert.Equal(t, c1, controller)
	})
}
