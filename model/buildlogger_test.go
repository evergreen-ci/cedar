package model

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/pail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestCreateLog(t *testing.T) {
	log, _ := getTestLogs()
	log.populated = true
	createdLog := CreateLog(log.Info, PailS3)
	assert.Equal(t, log.ID, createdLog.ID)
	assert.Equal(t, log.Info, createdLog.Info)
	assert.Equal(t, PailS3, createdLog.Artifact.Type)
	assert.Equal(t, log.ID, createdLog.Artifact.Prefix)
	assert.Equal(t, 0, createdLog.Artifact.Version)
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
		assert.Error(t, l.appendLogChunkInfo(LogChunkInfo{
			Key:      "key",
			NumLines: 100,
		}))
	})
	t.Run("DNE", func(t *testing.T) {
		l := &Log{ID: "DNE"}
		l.Setup(env)
		assert.Error(t, l.appendLogChunkInfo(LogChunkInfo{
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

		require.NoError(t, l.appendLogChunkInfo(chunks[0]))
		updatedLog := &Log{}
		require.NoError(t, db.Collection(buildloggerCollection).FindOne(ctx, bson.M{"_id": log1.ID}).Decode(updatedLog))
		assert.Equal(t, log1.ID, updatedLog.ID)
		assert.Equal(t, log1.Info, updatedLog.Info)
		assert.Equal(t, log1.Artifact.Prefix, updatedLog.Artifact.Prefix)
		assert.Equal(t, log1.Artifact.Version, updatedLog.Artifact.Version)
		assert.Equal(t, chunks[:1], updatedLog.Artifact.Chunks)

		l.Setup(env)
		require.NoError(t, l.appendLogChunkInfo(chunks[1]))
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

		require.NoError(t, l.appendLogChunkInfo(chunk))
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

func TestBuildloggerAppend(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := env.Context()
	defer cancel()
	tmpDir, err := ioutil.TempDir(".", "append-test")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
		assert.NoError(t, db.Collection(buildloggerCollection).Drop(ctx))
	}()

	testBucket, err := pail.NewLocalBucket(pail.LocalOptions{Path: tmpDir})
	require.NoError(t, err)

	log := Log{populated: true}
	log.ID = log.Info.ID()
	log.Artifact = LogArtifactInfo{
		Type:   PailLocal,
		Prefix: log.Info.ID(),
	}
	chunk1 := []LogLine{
		{
			Timestamp: time.Now().Add(-time.Hour).Round(time.Millisecond).UTC(),
			Data:      "This is not a test.\n",
		},
		{
			Timestamp: time.Now().Add(-59 * time.Minute).Round(time.Millisecond).UTC(),
			Data:      "This is a test.\n",
		},
	}
	chunk2 := []LogLine{
		{
			Timestamp: time.Now().Add(-57 * time.Minute).Round(time.Millisecond).UTC(),
			Data:      "Logging is fun.\n",
		},
		{
			Timestamp: time.Now().Add(-56 * time.Minute).Round(time.Millisecond).UTC(),
			Data:      "Buildogger logging logs.\n",
		},

		{
			Timestamp: time.Now().Add(-55 * time.Minute).Round(time.Millisecond).UTC(),
			Data:      "Finished logging.\n",
		},
	}

	t.Run("NoEnv", func(t *testing.T) {
		l := Log{populated: true}
		assert.Error(t, l.Append(chunk1))
	})
	t.Run("Unpopulated", func(t *testing.T) {
		l := Log{}
		l.Setup(env)
		assert.Error(t, l.Append(chunk1))
	})
	t.Run("DNE", func(t *testing.T) {
		log.Setup(env)
		assert.Error(t, log.Append(chunk1))
	})
	_, err = db.Collection(buildloggerCollection).InsertOne(ctx, log)
	require.NoError(t, err)
	t.Run("NoConfig", func(t *testing.T) {
		log.Setup(env)
		assert.Error(t, log.Append(chunk1))
	})
	conf := &CedarConfig{populated: true}
	conf.Setup(env)
	require.NoError(t, conf.Save())
	t.Run("ConfigWithoutBucket", func(t *testing.T) {
		log.Setup(env)
		assert.Error(t, log.Append(chunk1))
	})
	conf.Setup(env)
	require.NoError(t, conf.Find())
	conf.Bucket.BuildLogsBucket = tmpDir
	require.NoError(t, conf.Save())
	t.Run("AppendToBucketAndDB", func(t *testing.T) {
		log.Setup(env)
		require.NoError(t, log.Append(chunk1))
		require.NoError(t, log.Append(chunk2))
		expectedData := []byte{}
		for _, line := range append(chunk1, chunk2...) {
			expectedData = append(expectedData, []byte(fmt.Sprintf("%d%s", util.UnixMilli(line.Timestamp), line.Data))...)
		}

		filenames := map[string]bool{}
		actualData := []byte{}
		iter, err := testBucket.List(ctx, log.ID)
		require.NoError(t, err)
		for iter.Next(ctx) {
			key, err := filepath.Rel(log.ID, iter.Item().Name())
			require.NoError(t, err)
			filenames[key] = true
			r, err := iter.Item().Get(ctx)
			require.NoError(t, err)
			defer func() {
				assert.NoError(t, r.Close())
			}()
			data, err := ioutil.ReadAll(r)
			require.NoError(t, err)
			actualData = append(actualData, data...)
		}
		assert.Equal(t, expectedData, actualData)
		assert.Len(t, filenames, 2)

		l := &Log{}
		require.NoError(t, db.Collection(buildloggerCollection).FindOne(ctx, bson.M{"_id": log.Info.ID()}).Decode(l))
		assert.Len(t, l.Artifact.Chunks, 2)
		chunks := [][]LogLine{chunk1, chunk2}
		for i, chunk := range l.Artifact.Chunks {
			assert.True(t, filenames[chunk.Key])
			assert.Equal(t, len(chunks[i]), chunk.NumLines)
			assert.Equal(t, chunks[i][0].Timestamp, chunk.Start)
			assert.Equal(t, chunks[i][len(chunks[i])-1].Timestamp, chunk.End)
		}
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
