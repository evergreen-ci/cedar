package model

import (
	"context"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/pail"
	"github.com/evergreen-ci/utility"
	"github.com/jpillora/backoff"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
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

func TestSystemMetricsAppend(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tmpDir, err := ioutil.TempDir(".", "append-test")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
		assert.NoError(t, db.Collection(systemMetricsCollection).Drop(ctx))
		assert.NoError(t, db.Collection(configurationCollection).Drop(ctx))
	}()

	testBucket, err := pail.NewLocalBucket(pail.LocalOptions{Path: tmpDir})
	require.NoError(t, err)

	systemMetrics := getSystemMetrics()
	systemMetrics.populated = true

	chunk1 := make([]byte, 32)
	chunk2 := make([]byte, 32)
	rand.Read(chunk1)
	rand.Read(chunk2)

	t.Run("NoEnv", func(t *testing.T) {
		sm := SystemMetrics{ID: systemMetrics.ID}
		assert.Error(t, sm.Append(ctx, 1, chunk1))
	})
	t.Run("DNE", func(t *testing.T) {
		systemMetrics.Setup(env)
		assert.Error(t, systemMetrics.Append(ctx, 1, chunk1))
	})
	_, err = db.Collection(systemMetricsCollection).InsertOne(ctx, systemMetrics)
	require.NoError(t, err)
	t.Run("NoConfig", func(t *testing.T) {
		systemMetrics.Setup(env)
		assert.Error(t, systemMetrics.Append(ctx, 1, chunk1))
	})
	conf := &CedarConfig{populated: true}
	conf.Setup(env)
	require.NoError(t, conf.Save())
	t.Run("ConfigWithoutBucket", func(t *testing.T) {
		systemMetrics.Setup(env)
		assert.Error(t, systemMetrics.Append(ctx, 1, chunk1))
	})
	conf.Setup(env)
	require.NoError(t, conf.Find())
	conf.Bucket.SystemMetricsBucket = tmpDir
	require.NoError(t, conf.Save())
	t.Run("AppendToBucketAndDB", func(t *testing.T) {
		systemMetrics.Setup(env)
		require.NoError(t, systemMetrics.Append(ctx, 1, chunk1))
		require.NoError(t, systemMetrics.Append(ctx, 2, chunk2))
		expectedData := map[string][]byte{
			"1": chunk1,
			"2": chunk2,
		}

		b := &backoff.Backoff{
			Min:    100 * time.Millisecond,
			Max:    5 * time.Second,
			Factor: 2,
		}
		var actualData map[string][]byte
		for i := 0; i < 10; i++ {
			actualData = map[string][]byte{}
			iter, err := testBucket.List(ctx, systemMetrics.ID)
			require.NoError(t, err)
			for iter.Next(ctx) {
				key, err := filepath.Rel(systemMetrics.ID, iter.Item().Name())
				require.NoError(t, err)
				r, err := iter.Item().Get(ctx)
				require.NoError(t, err)
				defer func() {
					assert.NoError(t, r.Close())
				}()
				data, err := ioutil.ReadAll(r)
				require.NoError(t, err)
				actualData[key] = data
			}

			if len(actualData) > 1 {
				break
			}
			time.Sleep(b.Duration())
		}
		assert.Equal(t, expectedData, actualData)

		sm := &SystemMetrics{}
		require.NoError(t, db.Collection(systemMetricsCollection).FindOne(ctx, bson.M{"_id": systemMetrics.Info.ID()}).Decode(sm))
		assert.Len(t, sm.Artifact.Chunks, 2)
		for _, key := range sm.Artifact.Chunks {
			_, ok := actualData[key]
			assert.True(t, ok)
		}
	})
}

func TestSystemMetricsAppendChunkKey(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(systemMetricsCollection).Drop(ctx))
	}()
	systemMetrics1 := getSystemMetrics()
	systemMetrics2 := getSystemMetrics()
	systemMetrics2.Artifact.Chunks = []string{"1", "2"}

	_, err := db.Collection(systemMetricsCollection).InsertOne(ctx, systemMetrics1)
	require.NoError(t, err)
	_, err = db.Collection(systemMetricsCollection).InsertOne(ctx, systemMetrics2)
	require.NoError(t, err)

	t.Run("NoEnv", func(t *testing.T) {
		sm := &SystemMetrics{ID: systemMetrics1.ID}
		assert.Error(t, sm.appendSystemMetricsChunkKey(ctx, "1"))
	})
	t.Run("DNE", func(t *testing.T) {
		sm := &SystemMetrics{ID: "DNE"}
		sm.Setup(env)
		assert.Error(t, sm.appendSystemMetricsChunkKey(ctx, "1"))
	})
	t.Run("PushToEmptyArray", func(t *testing.T) {
		chunks := []string{"1", "2"}
		sm := &SystemMetrics{ID: systemMetrics1.ID}
		sm.Setup(env)

		require.NoError(t, sm.appendSystemMetricsChunkKey(ctx, chunks[0]))
		updatedSystemMetrics := &SystemMetrics{}
		require.NoError(t, db.Collection(systemMetricsCollection).FindOne(ctx, bson.M{"_id": systemMetrics1.ID}).Decode(updatedSystemMetrics))
		assert.Equal(t, systemMetrics1.ID, updatedSystemMetrics.ID)
		assert.Equal(t, systemMetrics1.Info, updatedSystemMetrics.Info)
		assert.Equal(t, systemMetrics1.Artifact.Prefix, updatedSystemMetrics.Artifact.Prefix)
		assert.Equal(t, chunks[:1], updatedSystemMetrics.Artifact.Chunks)

		sm.Setup(env)
		require.NoError(t, sm.appendSystemMetricsChunkKey(ctx, chunks[1]))
		require.NoError(t, db.Collection(systemMetricsCollection).FindOne(ctx, bson.M{"_id": systemMetrics1.ID}).Decode(updatedSystemMetrics))
		assert.Equal(t, systemMetrics1.ID, updatedSystemMetrics.ID)
		assert.Equal(t, systemMetrics1.Info, updatedSystemMetrics.Info)
		assert.Equal(t, systemMetrics1.Artifact.Prefix, updatedSystemMetrics.Artifact.Prefix)
		assert.Equal(t, chunks, updatedSystemMetrics.Artifact.Chunks)
	})
	t.Run("PushToExistingArray", func(t *testing.T) {
		sm := &SystemMetrics{ID: systemMetrics2.ID}
		sm.Setup(env)

		require.NoError(t, sm.appendSystemMetricsChunkKey(ctx, "3"))
		updatedSystemMetrics := &SystemMetrics{}
		require.NoError(t, db.Collection(systemMetricsCollection).FindOne(ctx, bson.M{"_id": systemMetrics2.ID}).Decode(updatedSystemMetrics))
		assert.Equal(t, systemMetrics2.ID, updatedSystemMetrics.ID)
		assert.Equal(t, systemMetrics2.Info, updatedSystemMetrics.Info)
		assert.Equal(t, systemMetrics2.Artifact.Prefix, updatedSystemMetrics.Artifact.Prefix)
		assert.Equal(t, append(systemMetrics2.Artifact.Chunks, "3"), updatedSystemMetrics.Artifact.Chunks)

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
			Chunks: []string{},
			Options: SystemMetricsArtifactOptions{
				Type:        PailLocal,
				Format:      FileFTDC,
				Compression: FileUncompressed,
				Schema:      SchemaRawEvents,
			},
		},
	}
}
