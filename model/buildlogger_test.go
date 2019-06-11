package model

import (
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/pail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
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

func TestBuildloggerSave(t *testing.T) {
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
		assert.Error(t, l.Save())
	})
	t.Run("Unpopulated", func(t *testing.T) {
		l := Log{
			ID:        log1.ID,
			Info:      log1.Info,
			Artifact:  log1.Artifact,
			populated: false,
		}
		l.Setup(env)
		assert.Error(t, l.Save())
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
		require.NoError(t, l.Save())
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
		require.NoError(t, l.Save())
		require.NoError(t, db.Collection(buildloggerCollection).FindOne(ctx, bson.M{"_id": log2.ID}).Decode(savedLog))
		assert.Equal(t, log2.ID, savedLog.ID)
		assert.Equal(t, log2.Info, savedLog.Info)
		assert.Equal(t, log2.Artifact, savedLog.Artifact)
	})
	t.Run("Upsert", func(t *testing.T) {
		savedLog := &Log{}

		l := Log{
			ID:        log2.ID,
			populated: true,
			Artifact:  log2.Artifact,
		}
		l.Setup(env)
		require.NoError(t, l.Save())
		require.NoError(t, db.Collection(buildloggerCollection).FindOne(ctx, bson.M{"_id": log2.ID}).Decode(savedLog))
		assert.Equal(t, log2.ID, savedLog.ID)

		l.Artifact.Prefix = "changedPrefix"
		l.Artifact.Chunks = []LogChunkInfo{
			log2.Artifact.Chunks[0],
			{
				Key:      "key3",
				NumLines: 500,
			},
		}
		l.populated = true
		l.Setup(env)
		require.NoError(t, l.Save())
		require.NoError(t, db.Collection(buildloggerCollection).FindOne(ctx, bson.M{"_id": log2.ID}).Decode(savedLog))
		assert.Equal(t, l.Artifact.Prefix, savedLog.Artifact.Prefix)
		assert.Equal(t, append(log2.Artifact.Chunks, l.Artifact.Chunks[1]), savedLog.Artifact.Chunks)
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
			Prefix:      "log1",
			Permissions: pail.S3PermissionsPublicRead,
			Version:     1,
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
			Prefix:      "log2",
			Permissions: pail.S3PermissionsPublicRead,
			Version:     1,
			Chunks: []LogChunkInfo{
				{
					Key:      "key1",
					NumLines: 100,
					Start:    time.Now().Add(-2 * time.Hour).Unix(),
					End:      time.Now().Add(-90 * time.Minute).Unix(),
				},
				{
					Key:      "key2",
					NumLines: 101,
					Start:    time.Now().Add(-89 * time.Minute).Unix(),
					End:      time.Now().Add(-time.Hour).Unix(),
				},
			},
		},
	}
	log2.ID = log2.Info.ID()

	return log1, log2
}
