package model

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/pail"
	"github.com/evergreen-ci/utility"
	"github.com/jpillora/backoff"
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

	chunk1 := []byte(utility.RandomString())
	chunk2 := []byte(utility.RandomString())

	t.Run("NoEnv", func(t *testing.T) {
		sm := SystemMetrics{ID: systemMetrics.ID}
		assert.Error(t, sm.Append(ctx, "Test", chunk1))
	})
	t.Run("DNE", func(t *testing.T) {
		systemMetrics.Setup(env)
		assert.Error(t, systemMetrics.Append(ctx, "Test", chunk1))
	})
	_, err = db.Collection(systemMetricsCollection).InsertOne(ctx, systemMetrics)
	require.NoError(t, err)
	t.Run("NoConfig", func(t *testing.T) {
		systemMetrics.Setup(env)
		assert.Error(t, systemMetrics.Append(ctx, "Test", chunk1))
	})
	conf := &CedarConfig{populated: true}
	conf.Setup(env)
	require.NoError(t, conf.Save())
	t.Run("ConfigWithoutBucket", func(t *testing.T) {
		systemMetrics.Setup(env)
		assert.Error(t, systemMetrics.Append(ctx, "Test", chunk1))
	})
	conf.Setup(env)
	require.NoError(t, conf.Find())
	conf.Bucket.SystemMetricsBucket = tmpDir
	require.NoError(t, conf.Save())
	t.Run("AppendToBucketAndDB", func(t *testing.T) {
		systemMetrics.Setup(env)
		require.NoError(t, systemMetrics.Append(ctx, "Test", chunk1))
		require.NoError(t, systemMetrics.Append(ctx, "Test", chunk2))

		b := &backoff.Backoff{
			Min:    100 * time.Millisecond,
			Max:    5 * time.Second,
			Factor: 2,
		}
		var keyCheck map[string]string
		for i := 0; i < 10; i++ {
			keyCheck = map[string]string{}
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
				keyCheck[string(data)] = key
			}

			if len(keyCheck) > 1 {
				break
			}
			time.Sleep(b.Duration())
		}
		chunk1Key, ok1 := keyCheck[string(chunk1)]
		chunk2Key, ok2 := keyCheck[string(chunk2)]
		assert.True(t, ok1 && ok2)
		assert.Equal(t, "Test-", chunk1Key[:5])
		assert.Equal(t, "Test-", chunk2Key[:5])
		chunk1Nanos, err1 := strconv.ParseInt(chunk1Key[5:], 10, 64)
		chunk2Nanos, err2 := strconv.ParseInt(chunk2Key[5:], 10, 64)
		assert.NoError(t, err1, err2)
		assert.True(t, chunk1Nanos < chunk2Nanos)

		sm := &SystemMetrics{}
		require.NoError(t, db.Collection(systemMetricsCollection).FindOne(ctx, bson.M{"_id": systemMetrics.Info.ID()}).Decode(sm))
		assert.Len(t, sm.Artifact.Chunks["Test"], 2)
		assert.Equal(t, sm.Artifact.Chunks["Test"], []string{chunk1Key, chunk2Key})
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

	key1 := "Test-" + fmt.Sprint(utility.UnixMilli(time.Now().Add(-20*time.Second)))
	key2 := "Test-" + fmt.Sprint(utility.UnixMilli(time.Now().Add(-10*time.Second)))

	systemMetrics1 := getSystemMetrics()
	systemMetrics2 := getSystemMetrics()
	systemMetrics2.Artifact.Chunks = map[string][]string{
		"Test": {key1, key2},
	}

	_, err := db.Collection(systemMetricsCollection).InsertOne(ctx, systemMetrics1)
	require.NoError(t, err)
	_, err = db.Collection(systemMetricsCollection).InsertOne(ctx, systemMetrics2)
	require.NoError(t, err)

	t.Run("NoEnv", func(t *testing.T) {
		sm := &SystemMetrics{ID: systemMetrics1.ID}
		assert.Error(t, sm.appendSystemMetricsChunkKey(ctx, "Test", key1))
	})
	t.Run("DNE", func(t *testing.T) {
		sm := &SystemMetrics{ID: "DNE"}
		sm.Setup(env)
		assert.Error(t, sm.appendSystemMetricsChunkKey(ctx, "Test", key1))
	})
	t.Run("PushToEmptyArray", func(t *testing.T) {
		chunks := map[string][]string{
			"Test": {key1, key2},
		}
		sm := &SystemMetrics{ID: systemMetrics1.ID}
		sm.Setup(env)

		require.NoError(t, sm.appendSystemMetricsChunkKey(ctx, "Test", chunks["Test"][0]))
		updatedSystemMetrics := &SystemMetrics{}
		require.NoError(t, db.Collection(systemMetricsCollection).FindOne(ctx, bson.M{"_id": systemMetrics1.ID}).Decode(updatedSystemMetrics))
		assert.Equal(t, systemMetrics1.ID, updatedSystemMetrics.ID)
		assert.Equal(t, systemMetrics1.Info, updatedSystemMetrics.Info)
		assert.Equal(t, systemMetrics1.Artifact.Prefix, updatedSystemMetrics.Artifact.Prefix)
		assert.Equal(t, chunks["Test"][:1], updatedSystemMetrics.Artifact.Chunks["Test"])

		sm.Setup(env)
		require.NoError(t, sm.appendSystemMetricsChunkKey(ctx, "Test", chunks["Test"][1]))
		require.NoError(t, db.Collection(systemMetricsCollection).FindOne(ctx, bson.M{"_id": systemMetrics1.ID}).Decode(updatedSystemMetrics))
		assert.Equal(t, systemMetrics1.ID, updatedSystemMetrics.ID)
		assert.Equal(t, systemMetrics1.Info, updatedSystemMetrics.Info)
		assert.Equal(t, systemMetrics1.Artifact.Prefix, updatedSystemMetrics.Artifact.Prefix)
		assert.Equal(t, chunks["Test"], updatedSystemMetrics.Artifact.Chunks["Test"])
	})
	t.Run("PushToExistingArray", func(t *testing.T) {
		sm := &SystemMetrics{ID: systemMetrics2.ID}
		sm.Setup(env)

		key3 := "Test" + fmt.Sprint(utility.UnixMilli(time.Now()))

		require.NoError(t, sm.appendSystemMetricsChunkKey(ctx, "Test", key3))
		updatedSystemMetrics := &SystemMetrics{}
		require.NoError(t, db.Collection(systemMetricsCollection).FindOne(ctx, bson.M{"_id": systemMetrics2.ID}).Decode(updatedSystemMetrics))
		assert.Equal(t, systemMetrics2.ID, updatedSystemMetrics.ID)
		assert.Equal(t, systemMetrics2.Info, updatedSystemMetrics.Info)
		assert.Equal(t, systemMetrics2.Artifact.Prefix, updatedSystemMetrics.Artifact.Prefix)
		assert.Equal(t, append(systemMetrics2.Artifact.Chunks["Test"], key3), updatedSystemMetrics.Artifact.Chunks["Test"])

		key4 := "DifferentType" + fmt.Sprint(utility.UnixMilli(time.Now()))
		require.NoError(t, sm.appendSystemMetricsChunkKey(ctx, "DifferentType", key4))
		updatedSystemMetrics = &SystemMetrics{}
		require.NoError(t, db.Collection(systemMetricsCollection).FindOne(ctx, bson.M{"_id": systemMetrics2.ID}).Decode(updatedSystemMetrics))
		assert.Equal(t, systemMetrics2.ID, updatedSystemMetrics.ID)
		assert.Equal(t, systemMetrics2.Info, updatedSystemMetrics.Info)
		assert.Equal(t, systemMetrics2.Artifact.Prefix, updatedSystemMetrics.Artifact.Prefix)
		assert.Equal(t, 1, len(updatedSystemMetrics.Artifact.Chunks["DifferentType"]))
		assert.Equal(t, []string{key4}, updatedSystemMetrics.Artifact.Chunks["DifferentType"])
	})
}

func TestSystemMetricsRemove(t *testing.T) {
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

	t.Run("NoEnv", func(t *testing.T) {
		sm := SystemMetrics{
			ID: sm1.ID,
		}
		assert.Error(t, sm.Remove(ctx))
	})
	t.Run("DNE", func(t *testing.T) {
		sm := SystemMetrics{ID: "DNE"}
		sm.Setup(env)
		require.NoError(t, sm.Remove(ctx))
	})
	t.Run("WithID", func(t *testing.T) {
		sm := SystemMetrics{ID: sm1.ID}
		sm.Setup(env)
		require.NoError(t, sm.Remove(ctx))

		saved := &SystemMetrics{}
		require.Error(t, db.Collection(systemMetricsCollection).FindOne(ctx, bson.M{"_id": sm1.ID}).Decode(saved))
	})
	t.Run("WithoutID", func(t *testing.T) {
		sm := SystemMetrics{Info: sm2.Info}
		sm.Setup(env)
		require.NoError(t, sm.Remove(ctx))

		saved := &SystemMetrics{}
		require.Error(t, db.Collection(systemMetricsCollection).FindOne(ctx, bson.M{"_id": sm2.ID}).Decode(saved))
	})
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

func TestSystemMetricsClose(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := env.Context()
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(systemMetricsCollection).Drop(ctx))
	}()
	sm1 := getSystemMetrics()
	sm2 := getSystemMetrics()

	_, err := db.Collection(systemMetricsCollection).InsertOne(ctx, sm1)
	require.NoError(t, err)
	_, err = db.Collection(systemMetricsCollection).InsertOne(ctx, sm2)
	require.NoError(t, err)

	t.Run("NoEnv", func(t *testing.T) {
		sm := &SystemMetrics{ID: sm1.ID, populated: true}
		assert.Error(t, sm.Close(ctx, true))
	})
	t.Run("DNE", func(t *testing.T) {
		sm := &SystemMetrics{ID: "DNE"}
		sm.Setup(env)
		assert.Error(t, sm.Close(ctx, true))
	})
	t.Run("WithID", func(t *testing.T) {
		l1 := &SystemMetrics{ID: sm1.ID, populated: true}
		l1.Setup(env)
		require.NoError(t, l1.Close(ctx, true))

		updatedSystemMetrics := &SystemMetrics{}
		require.NoError(t, db.Collection(systemMetricsCollection).FindOne(ctx, bson.M{"_id": sm1.ID}).Decode(updatedSystemMetrics))
		assert.Equal(t, sm1.ID, updatedSystemMetrics.ID)
		assert.Equal(t, sm1.ID, updatedSystemMetrics.Info.ID())
		assert.WithinDuration(t, sm1.CreatedAt.UTC(), updatedSystemMetrics.CreatedAt, time.Second)
		assert.True(t, time.Since(updatedSystemMetrics.CompletedAt) <= time.Second)
		assert.Equal(t, sm1.Info.Mainline, updatedSystemMetrics.Info.Mainline)
		assert.Equal(t, sm1.Info.Schema, updatedSystemMetrics.Info.Schema)
		assert.Equal(t, sm1.Artifact, updatedSystemMetrics.Artifact)
		assert.Equal(t, true, updatedSystemMetrics.Info.Success)
	})
	t.Run("WithoutID", func(t *testing.T) {
		l2 := &SystemMetrics{Info: sm2.Info, populated: true}
		l2.Setup(env)
		require.NoError(t, l2.Close(ctx, true))

		updatedSystemMetrics := &SystemMetrics{}
		require.NoError(t, db.Collection(systemMetricsCollection).FindOne(ctx, bson.M{"_id": sm2.ID}).Decode(updatedSystemMetrics))
		assert.Equal(t, sm2.ID, updatedSystemMetrics.ID)
		assert.Equal(t, sm2.ID, updatedSystemMetrics.Info.ID())
		assert.WithinDuration(t, sm2.CreatedAt.UTC(), updatedSystemMetrics.CreatedAt, time.Second)
		assert.True(t, time.Since(updatedSystemMetrics.CompletedAt) <= time.Second)
		assert.Equal(t, sm2.Info.Mainline, updatedSystemMetrics.Info.Mainline)
		assert.Equal(t, sm2.Info.Schema, updatedSystemMetrics.Info.Schema)
		assert.Equal(t, sm2.Artifact, updatedSystemMetrics.Artifact)
		assert.Equal(t, true, updatedSystemMetrics.Info.Success)
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
		Success:  true,
	}
	return &SystemMetrics{
		ID:          info.ID(),
		Info:        info,
		CreatedAt:   time.Now().Add(-time.Hour).UTC().Round(time.Millisecond),
		CompletedAt: time.Now().UTC().Round(time.Millisecond),
		Artifact: SystemMetricsArtifactInfo{
			Prefix: info.ID(),
			Chunks: map[string][]string{},
			Options: SystemMetricsArtifactOptions{
				Type:        PailLocal,
				Format:      FileFTDC,
				Compression: FileUncompressed,
				Schema:      SchemaRawEvents,
			},
		},
	}
}
