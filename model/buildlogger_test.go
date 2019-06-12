package model

import (
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestCreateLog(t *testing.T) {
	log, _ := getTestLogs()
	log.populated = true
	createdLog := CreateLog(log.Info, log.Artifact)
	assert.Equal(t, log.ID, createdLog.ID)
	assert.Equal(t, log.Info, createdLog.Info)
	assert.Equal(t, log.Artifact, createdLog.Artifact)
	assert.True(t, createdLog.populated)
}

func TestBuildloggerFind(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := env.Context()
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(buildloggerCollection).Drop(ctx))
	}()
	log1, log2 := getTestLogs()

	_, err := db.Collection(buildloggerCollection).InsertOne(ctx, log1)
	require.NoError(t, err)
	_, err = db.Collection(buildloggerCollection).InsertOne(ctx, log2)
	require.NoError(t, err)

	t.Run("DNE", func(t *testing.T) {
		l := Log{ID: "DNE"}
		l.Setup(env)
		assert.Error(t, l.Find())
	})
	t.Run("NoEnv", func(t *testing.T) {
		l := Log{ID: log1.ID}
		assert.Error(t, l.Find())
	})
	t.Run("WithID", func(t *testing.T) {
		l := Log{ID: log1.ID}
		l.Setup(env)
		require.NoError(t, l.Find())
		assert.Equal(t, log1.ID, l.ID)
		assert.Equal(t, log1.Info, l.Info)
		assert.Equal(t, log1.Artifact, l.Artifact)
		assert.True(t, l.populated)
	})
	t.Run("WithoutID", func(t *testing.T) {
		l := Log{Info: log2.Info}
		l.Setup(env)
		require.NoError(t, l.Find())
		assert.Equal(t, log2.ID, l.ID)
		assert.Equal(t, log2.Info, l.Info)
		assert.Equal(t, log2.Artifact, l.Artifact)
		assert.True(t, l.populated)

	})
}

func TestBuildloggerSaveNew(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := env.Context()
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(buildloggerCollection).Drop(ctx))
	}()
	log1, log2 := getTestLogs()

	t.Run("NoEnv", func(t *testing.T) {
		l := Log{
			ID:        log1.ID,
			Info:      log1.Info,
			Artifact:  log1.Artifact,
			populated: true,
		}
		assert.Error(t, l.SaveNew())
	})
	t.Run("Unpopulated", func(t *testing.T) {
		l := Log{
			ID:        log1.ID,
			Info:      log1.Info,
			Artifact:  log1.Artifact,
			populated: false,
		}
		l.Setup(env)
		assert.Error(t, l.SaveNew())
	})
	t.Run("WithID", func(t *testing.T) {
		savedLog := &Log{}
		require.Error(t, db.Collection(buildloggerCollection).FindOne(ctx, bson.M{"_id": log1.ID}).Decode(savedLog))

		l := Log{
			ID:        log1.ID,
			Info:      log1.Info,
			Artifact:  log1.Artifact,
			populated: true,
		}
		l.Setup(env)
		require.NoError(t, l.SaveNew())
		require.NoError(t, db.Collection(buildloggerCollection).FindOne(ctx, bson.M{"_id": log1.ID}).Decode(savedLog))
		assert.Equal(t, log1.ID, savedLog.ID)
		assert.Equal(t, log1.Info, savedLog.Info)
		assert.Equal(t, log1.Artifact, savedLog.Artifact)
	})
	t.Run("WithoutID", func(t *testing.T) {
		savedLog := &Log{}
		require.Error(t, db.Collection(buildloggerCollection).FindOne(ctx, bson.M{"_id": log2.ID}).Decode(savedLog))

		l := Log{
			Info:      log2.Info,
			Artifact:  log2.Artifact,
			populated: true,
		}
		l.Setup(env)
		require.NoError(t, l.SaveNew())
		require.NoError(t, db.Collection(buildloggerCollection).FindOne(ctx, bson.M{"_id": log2.ID}).Decode(savedLog))
		assert.Equal(t, log2.ID, savedLog.ID)
		assert.Equal(t, log2.Info, savedLog.Info)
		assert.Equal(t, log2.Artifact, savedLog.Artifact)
	})
	t.Run("AlreadyExists", func(t *testing.T) {
		_, err := db.Collection(buildloggerCollection).ReplaceOne(ctx, bson.M{"_id": log2.ID}, log2, options.Replace().SetUpsert(true))
		require.NoError(t, err)

		l := Log{
			ID:        log2.ID,
			populated: true,
		}
		l.Setup(env)
		require.Error(t, l.SaveNew())
	})
}

func TestBuildloggerAppendLogChunkInfo(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := env.Context()
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(buildloggerCollection).Drop(ctx))
	}()
	log1, log2 := getTestLogs()

	_, err := db.Collection(buildloggerCollection).InsertOne(ctx, log1)
	require.NoError(t, err)
	_, err = db.Collection(buildloggerCollection).InsertOne(ctx, log2)
	require.NoError(t, err)

	t.Run("NoEnv", func(t *testing.T) {
		l := &Log{ID: log1.ID}
		assert.Error(t, l.AppendLogChunkInfo(LogChunkInfo{
			Key:      "key",
			NumLines: 100,
		}))
	})
	t.Run("DNE", func(t *testing.T) {
		l := &Log{ID: "DNE"}
		l.Setup(env)
		assert.Error(t, l.AppendLogChunkInfo(LogChunkInfo{
			Key:      "key",
			NumLines: 100,
		}))
	})
	t.Run("PushToEmptyArray", func(t *testing.T) {
		chunks := []LogChunkInfo{
			{
				Key:      "key1",
				NumLines: 100,
				Start:    time.Date(2019, time.January, 1, 12, 0, 0, 0, time.UTC),
				End:      time.Date(2019, time.January, 1, 13, 30, 0, 0, time.UTC),
			},
			{
				Key:      "key2",
				NumLines: 101,
				Start:    time.Date(2019, time.January, 1, 13, 31, 0, 0, time.UTC),
				End:      time.Date(2019, time.January, 1, 14, 31, 0, 0, time.UTC),
			},
		}
		l := &Log{ID: log1.ID}
		l.Setup(env)

		require.NoError(t, l.AppendLogChunkInfo(chunks[0]))
		updatedLog := &Log{}
		require.NoError(t, db.Collection(buildloggerCollection).FindOne(ctx, bson.M{"_id": log1.ID}).Decode(updatedLog))
		assert.Equal(t, log1.ID, updatedLog.ID)
		assert.Equal(t, log1.Info, updatedLog.Info)
		assert.Equal(t, log1.Artifact.Prefix, updatedLog.Artifact.Prefix)
		assert.Equal(t, log1.Artifact.Version, updatedLog.Artifact.Version)
		assert.Equal(t, chunks[:1], updatedLog.Artifact.Chunks)

		l.Setup(env)
		require.NoError(t, l.AppendLogChunkInfo(chunks[1]))
		require.NoError(t, db.Collection(buildloggerCollection).FindOne(ctx, bson.M{"_id": log1.ID}).Decode(updatedLog))
		assert.Equal(t, log1.ID, updatedLog.ID)
		assert.Equal(t, log1.Info, updatedLog.Info)
		assert.Equal(t, log1.Artifact.Prefix, updatedLog.Artifact.Prefix)
		assert.Equal(t, log1.Artifact.Version, updatedLog.Artifact.Version)
		assert.Equal(t, chunks, updatedLog.Artifact.Chunks)
	})
	t.Run("PushToExistingArray", func(t *testing.T) {
		chunk := LogChunkInfo{
			Key:      "key3",
			NumLines: 1000,
		}
		l := &Log{ID: log2.ID}
		l.Setup(env)

		require.NoError(t, l.AppendLogChunkInfo(chunk))
		updatedLog := &Log{}
		require.NoError(t, db.Collection(buildloggerCollection).FindOne(ctx, bson.M{"_id": log2.ID}).Decode(updatedLog))
		assert.Equal(t, log2.ID, updatedLog.ID)
		assert.Equal(t, log2.Info, updatedLog.Info)
		assert.Equal(t, log2.Artifact.Prefix, updatedLog.Artifact.Prefix)
		assert.Equal(t, log2.Artifact.Version, updatedLog.Artifact.Version)
		assert.Equal(t, append(log2.Artifact.Chunks, chunk), updatedLog.Artifact.Chunks)

	})
}

func TestBuildloggerRemove(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := env.Context()
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(buildloggerCollection).Drop(ctx))
	}()
	log1, log2 := getTestLogs()

	_, err := db.Collection(buildloggerCollection).InsertOne(ctx, log1)
	require.NoError(t, err)
	_, err = db.Collection(buildloggerCollection).InsertOne(ctx, log2)
	require.NoError(t, err)

	t.Run("DNE", func(t *testing.T) {
		l := Log{ID: "DNE"}
		l.Setup(env)
		require.NoError(t, l.Remove())
	})
	t.Run("WithID", func(t *testing.T) {
		l := Log{ID: log1.ID}
		l.Setup(env)
		require.NoError(t, l.Remove())

		savedLog := &Log{}
		require.Error(t, db.Collection(buildloggerCollection).FindOne(ctx, bson.M{"_id": log1.ID}).Decode(savedLog))
	})
	t.Run("WithoutID", func(t *testing.T) {
		l := Log{ID: log2.ID}
		l.Setup(env)
		require.NoError(t, l.Remove())

		savedLog := &Log{}
		require.Error(t, db.Collection(buildloggerCollection).FindOne(ctx, bson.M{"_id": log2.ID}).Decode(savedLog))
	})
}

func getTestLogs() (*Log, *Log) {
	log1 := &Log{
		Info: LogInfo{
			Project:  "project",
			TestName: "test1",
		},
		CreatedAt:   time.Now().Add(-24 * time.Hour),
		CompletedAt: time.Now().Add(-23 * time.Hour),
		Artifact: LogArtifactInfo{
			Type:    PailS3,
			Prefix:  "log1",
			Version: 1,
		},
	}
	log1.ID = log1.Info.ID()
	log2 := &Log{
		Info: LogInfo{
			Project:  "project",
			TestName: "test2",
		},
		CreatedAt:   time.Now().Add(-2 * time.Hour),
		CompletedAt: time.Now().Add(-time.Hour),
		Artifact: LogArtifactInfo{
			Type:    PailS3,
			Prefix:  "log2",
			Version: 1,
			Chunks: []LogChunkInfo{
				{
					Key:      "key1",
					NumLines: 100,
					Start:    time.Date(2019, time.January, 1, 12, 0, 0, 0, time.UTC),
					End:      time.Date(2019, time.January, 1, 13, 30, 0, 0, time.UTC),
				},
				{
					Key:      "key2",
					NumLines: 101,
					Start:    time.Date(2019, time.January, 1, 13, 31, 0, 0, time.UTC),
					End:      time.Date(2019, time.January, 1, 14, 31, 0, 0, time.UTC),
				},
			},
		},
	}
	log2.ID = log2.Info.ID()

	return log1, log2
}
