package model

import (
	"context"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/utility"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestCreateSystemMetrics(t *testing.T) {
	expected := getSystemMetrics()
	expected.populated = true
	actual := CreateSystemMetrics(expected.Info, expected.Artifact.Options)
	assert.Equal(t, expected.ID, actual.ID)
	assert.Equal(t, expected.Info, actual.Info)
	assert.Equal(t, expected.Artifact, actual.Artifact)
	assert.True(t, time.Since(actual.CreatedAt) <= time.Second)
	assert.Zero(t, actual.CompletedAt)
	assert.True(t, actual.populated)
}

func TestSystemMetricsFind(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(systemMetricsCollection).Drop(ctx))
	}()

	sm1 := getSystemMetrics()
	_, err := db.Collection(systemMetricsCollection).InsertOne(ctx, sm1)
	require.NoError(t, err)
	sm2 := getSystemMetrics()
	_, err = db.Collection(systemMetricsCollection).InsertOne(ctx, sm2)
	require.NoError(t, err)

	t.Run("DNE", func(t *testing.T) {
		sm := SystemMetrics{ID: "DNE"}
		sm.Setup(env)
		assert.Error(t, sm.Find(ctx))
		assert.False(t, sm.populated)
	})
	t.Run("NoEnv", func(t *testing.T) {
		sm := SystemMetrics{ID: sm1.ID}
		assert.Error(t, sm.Find(ctx))
		assert.False(t, sm.populated)
	})
	t.Run("WithID", func(t *testing.T) {
		sm := SystemMetrics{ID: sm1.ID}
		sm.Setup(env)
		require.NoError(t, sm.Find(ctx))
		assert.Equal(t, sm1.ID, sm.ID)
		assert.Equal(t, sm1.Info, sm.Info)
		assert.Equal(t, sm1.Artifact, sm.Artifact)
		assert.True(t, sm.populated)
	})
	t.Run("WithoutID", func(t *testing.T) {
		sm := SystemMetrics{Info: sm2.Info}
		sm.Setup(env)
		require.NoError(t, sm.Find(ctx))
		assert.Equal(t, sm2.ID, sm.ID)
		assert.Equal(t, sm2.Info, sm.Info)
		assert.Equal(t, sm2.Artifact, sm.Artifact)
		assert.True(t, sm.populated)
	})
}

func getSystemMetrics() *SystemMetrics {
	info := SystemMetricsInfo{
		Project:  utility.RandomString(),
		Version:  utility.RandomString(),
		Variant:  utility.RandomString(),
		TaskName: utility.RandomString(),
		TaskID:   utility.RandomString(),
		Mainline: true,
		Schema:   0,
	}
	return &SystemMetrics{
		ID:          info.ID(),
		Info:        info,
		CreatedAt:   time.Now().Add(-time.Hour).UTC().Round(time.Millisecond),
		CompletedAt: time.Now().UTC().Round(time.Millisecond),
		Artifact: SystemMetricsArtifactInfo{
			Prefix: info.ID(),
			Keys:   []string{},
			Options: SystemMetricsArtifactOptions{
				Type:        PailLocal,
				Format:      FileFTDC,
				Compression: FileUncompressed,
				Schema:      SchemaRawEvents,
			},
		},
	}
}

func TestSystemMetricsSaveNew(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(systemMetricsCollection).Drop(ctx))
	}()
	sm1 := getSystemMetrics()
	sm2 := getSystemMetrics()

	t.Run("NoEnv", func(t *testing.T) {
		sm := SystemMetrics{
			ID:        sm1.ID,
			Info:      sm1.Info,
			CreatedAt: sm1.CreatedAt,
			Artifact:  sm1.Artifact,
			populated: true,
		}
		assert.Error(t, sm.SaveNew(ctx))
	})
	t.Run("Unpopulated", func(t *testing.T) {
		sm := SystemMetrics{
			ID:        sm1.ID,
			Info:      sm1.Info,
			CreatedAt: sm1.CreatedAt,
			Artifact:  sm1.Artifact,
			populated: false,
		}
		sm.Setup(env)
		assert.Error(t, sm.SaveNew(ctx))
	})
	t.Run("WithID", func(t *testing.T) {
		saved := &SystemMetrics{}
		require.Error(t, db.Collection(systemMetricsCollection).FindOne(ctx, bson.M{"_id": sm1.ID}).Decode(saved))

		sm := SystemMetrics{
			ID:        sm1.ID,
			Info:      sm1.Info,
			CreatedAt: sm1.CreatedAt,
			Artifact:  sm1.Artifact,
			populated: true,
		}
		sm.Setup(env)
		require.NoError(t, sm.SaveNew(ctx))
		require.NoError(t, db.Collection(systemMetricsCollection).FindOne(ctx, bson.M{"_id": sm1.ID}).Decode(saved))
		assert.Equal(t, sm1.ID, saved.ID)
		assert.Equal(t, sm1.Info, saved.Info)
		assert.Equal(t, sm1.CreatedAt, saved.CreatedAt)
		assert.Equal(t, sm1.Artifact, saved.Artifact)
	})
	t.Run("WithoutID", func(t *testing.T) {
		saved := &SystemMetrics{}
		require.Error(t, db.Collection(systemMetricsCollection).FindOne(ctx, bson.M{"_id": sm2.ID}).Decode(saved))

		sm := SystemMetrics{
			Info:      sm2.Info,
			CreatedAt: sm2.CreatedAt,
			Artifact:  sm2.Artifact,
			populated: true,
		}
		sm.Setup(env)
		require.NoError(t, sm.SaveNew(ctx))
		require.NoError(t, db.Collection(systemMetricsCollection).FindOne(ctx, bson.M{"_id": sm2.ID}).Decode(saved))
		assert.Equal(t, sm2.ID, saved.ID)
		assert.Equal(t, sm2.Info, saved.Info)
		assert.Equal(t, sm2.CreatedAt, saved.CreatedAt)
		assert.Equal(t, sm2.Artifact, saved.Artifact)
	})
	t.Run("AlreadyExists", func(t *testing.T) {
		_, err := db.Collection(systemMetricsCollection).ReplaceOne(ctx, bson.M{"_id": sm2.ID}, sm2, options.Replace().SetUpsert(true))
		require.NoError(t, err)

		sm := SystemMetrics{
			ID:        sm2.ID,
			populated: true,
		}
		sm.Setup(env)
		require.Error(t, sm.SaveNew(ctx))
	})
}
