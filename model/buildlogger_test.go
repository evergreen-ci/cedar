package model

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/pail"
	"github.com/evergreen-ci/utility"
	"github.com/jpillora/backoff"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/grip/level"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestCreateLog(t *testing.T) {
	log, _ := getTestLogs(time.Now())
	log.populated = true
	createdLog := CreateLog(log.Info, PailS3)
	assert.Equal(t, log.ID, createdLog.ID)
	assert.Equal(t, log.Info, createdLog.Info)
	assert.True(t, time.Since(createdLog.CreatedAt) <= time.Second)
	assert.Equal(t, PailS3, createdLog.Artifact.Type)
	assert.Equal(t, log.ID, createdLog.Artifact.Prefix)
	assert.Equal(t, 0, createdLog.Artifact.Version)
	assert.True(t, createdLog.populated)
}

func TestBuildloggerFind(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(buildloggerCollection).Drop(ctx))
	}()
	log1, log2 := getTestLogs(time.Now())

	_, err := db.Collection(buildloggerCollection).InsertOne(ctx, log1)
	require.NoError(t, err)
	_, err = db.Collection(buildloggerCollection).InsertOne(ctx, log2)
	require.NoError(t, err)

	t.Run("DNE", func(t *testing.T) {
		l := Log{ID: "DNE"}
		l.Setup(env)
		assert.Error(t, l.Find(ctx))
	})
	t.Run("NoEnv", func(t *testing.T) {
		l := Log{ID: log1.ID}
		assert.Error(t, l.Find(ctx))
	})
	t.Run("WithID", func(t *testing.T) {
		l := Log{ID: log1.ID}
		l.Setup(env)
		require.NoError(t, l.Find(ctx))
		assert.Equal(t, log1.ID, l.ID)
		assert.Equal(t, log1.Info, l.Info)
		assert.Equal(t, log1.Artifact, l.Artifact)
		assert.True(t, l.populated)
	})
	t.Run("WithoutID", func(t *testing.T) {
		l := Log{Info: log2.Info}
		l.Setup(env)
		require.NoError(t, l.Find(ctx))
		assert.Equal(t, log2.ID, l.ID)
		assert.Equal(t, log2.Info, l.Info)
		assert.Equal(t, log2.Artifact, l.Artifact)
		assert.True(t, l.populated)

	})
}

func TestBuildloggerSaveNew(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(buildloggerCollection).Drop(ctx))
	}()
	log1, log2 := getTestLogs(time.Now())

	t.Run("NoEnv", func(t *testing.T) {
		l := Log{
			ID:        log1.ID,
			Info:      log1.Info,
			Artifact:  log1.Artifact,
			populated: true,
		}
		assert.Error(t, l.SaveNew(ctx))
	})
	t.Run("Unpopulated", func(t *testing.T) {
		l := Log{
			ID:        log1.ID,
			Info:      log1.Info,
			Artifact:  log1.Artifact,
			populated: false,
		}
		l.Setup(env)
		assert.Error(t, l.SaveNew(ctx))
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
		require.NoError(t, l.SaveNew(ctx))
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
		require.NoError(t, l.SaveNew(ctx))
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
		require.Error(t, l.SaveNew(ctx))
	})
}

func TestBuildloggerAppendLogChunkInfo(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(buildloggerCollection).Drop(ctx))
	}()
	log1, log2 := getTestLogs(time.Now())

	_, err := db.Collection(buildloggerCollection).InsertOne(ctx, log1)
	require.NoError(t, err)
	_, err = db.Collection(buildloggerCollection).InsertOne(ctx, log2)
	require.NoError(t, err)

	t.Run("NoEnv", func(t *testing.T) {
		l := &Log{ID: log1.ID}
		assert.Error(t, l.appendLogChunkInfo(ctx, LogChunkInfo{
			Key:      "key",
			NumLines: 100,
		}))
	})
	t.Run("DNE", func(t *testing.T) {
		l := &Log{ID: "DNE"}
		l.Setup(env)
		assert.Error(t, l.appendLogChunkInfo(ctx, LogChunkInfo{
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

		require.NoError(t, l.appendLogChunkInfo(ctx, chunks[0]))
		updatedLog := &Log{}
		require.NoError(t, db.Collection(buildloggerCollection).FindOne(ctx, bson.M{"_id": log1.ID}).Decode(updatedLog))
		assert.Equal(t, log1.ID, updatedLog.ID)
		assert.Equal(t, log1.Info, updatedLog.Info)
		assert.Equal(t, log1.Artifact.Prefix, updatedLog.Artifact.Prefix)
		assert.Equal(t, log1.Artifact.Version, updatedLog.Artifact.Version)
		assert.Equal(t, chunks[:1], updatedLog.Artifact.Chunks)

		l.Setup(env)
		require.NoError(t, l.appendLogChunkInfo(ctx, chunks[1]))
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

		require.NoError(t, l.appendLogChunkInfo(ctx, chunk))
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(buildloggerCollection).Drop(ctx))
	}()
	log1, log2 := getTestLogs(time.Now())

	_, err := db.Collection(buildloggerCollection).InsertOne(ctx, log1)
	require.NoError(t, err)
	_, err = db.Collection(buildloggerCollection).InsertOne(ctx, log2)
	require.NoError(t, err)

	t.Run("NoEnv", func(t *testing.T) {
		l := Log{
			ID: log1.ID,
		}
		assert.Error(t, l.Remove(ctx))
	})
	t.Run("DNE", func(t *testing.T) {
		l := Log{ID: "DNE"}
		l.Setup(env)
		require.NoError(t, l.Remove(ctx))
	})
	t.Run("WithID", func(t *testing.T) {
		l := Log{ID: log1.ID}
		l.Setup(env)
		require.NoError(t, l.Remove(ctx))

		savedLog := &Log{}
		require.Error(t, db.Collection(buildloggerCollection).FindOne(ctx, bson.M{"_id": log1.ID}).Decode(savedLog))
	})
	t.Run("WithoutID", func(t *testing.T) {
		l := Log{Info: log2.Info}
		l.Setup(env)
		require.NoError(t, l.Remove(ctx))

		savedLog := &Log{}
		require.Error(t, db.Collection(buildloggerCollection).FindOne(ctx, bson.M{"_id": log2.ID}).Decode(savedLog))
	})
}

func TestBuildloggerAppend(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tmpDir, err := ioutil.TempDir(".", "append-test")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
		assert.NoError(t, db.Collection(buildloggerCollection).Drop(ctx))
		assert.NoError(t, db.Collection(configurationCollection).Drop(ctx))
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
			Priority:  level.Debug,
			Timestamp: time.Now().Add(-time.Hour).Round(time.Millisecond).UTC(),
			Data:      "This is not a test.",
		},
		{
			Priority:  level.Emergency,
			Timestamp: time.Now().Add(-59 * time.Minute).Round(time.Millisecond).UTC(),
			Data:      "This is a test.",
		},
	}
	chunk2 := []LogLine{
		{
			Priority:  level.Info,
			Timestamp: time.Now().Add(-57 * time.Minute).Round(time.Millisecond).UTC(),
			Data:      "Logging is fun.",
		},
		{
			Priority:  level.Info,
			Timestamp: time.Now().Add(-56 * time.Minute).Round(time.Millisecond).UTC(),
			Data:      "Buildogger logging logs.",
		},

		{
			Priority:  level.Info,
			Timestamp: time.Now().Add(-55 * time.Minute).Round(time.Millisecond).UTC(),
			Data:      "Finished logging.",
		},
	}

	t.Run("NoEnv", func(t *testing.T) {
		l := Log{ID: log.ID}
		assert.Error(t, l.Append(ctx, chunk1))
	})
	t.Run("DNE", func(t *testing.T) {
		log.Setup(env)
		assert.Error(t, log.Append(ctx, chunk1))
	})
	_, err = db.Collection(buildloggerCollection).InsertOne(ctx, log)
	require.NoError(t, err)
	t.Run("NoConfig", func(t *testing.T) {
		log.Setup(env)
		assert.Error(t, log.Append(ctx, chunk1))
	})
	conf := &CedarConfig{populated: true}
	conf.Setup(env)
	require.NoError(t, conf.Save())
	t.Run("ConfigWithoutBucket", func(t *testing.T) {
		log.Setup(env)
		assert.Error(t, log.Append(ctx, chunk1))
	})
	conf.Setup(env)
	require.NoError(t, conf.Find())
	conf.Bucket.BuildLogsBucket = tmpDir
	require.NoError(t, conf.Save())
	t.Run("AppendToBucketAndDB", func(t *testing.T) {
		log.Setup(env)
		require.NoError(t, log.Append(ctx, chunk1))
		require.NoError(t, log.Append(ctx, chunk2))
		expectedData := []byte{}
		for _, line := range append(chunk1, chunk2...) {
			expectedData = append(expectedData, []byte(prependPriorityAndTimestamp(line.Priority, line.Timestamp, line.Data))...)
		}

		b := &backoff.Backoff{
			Min:    100 * time.Millisecond,
			Max:    5 * time.Second,
			Factor: 2,
		}
		var filenames map[string]bool
		var actualData []byte
		for i := 0; i < 20; i++ {
			filenames = map[string]bool{}
			actualData = []byte{}
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

			if len(filenames) > 1 {
				break
			}
			time.Sleep(b.Duration())
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

func TestBuildloggerDownload(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(buildloggerCollection).Drop(ctx))
		assert.NoError(t, db.Collection(configurationCollection).Drop(ctx))
	}()
	log1, log2 := getTestLogs(time.Now())

	_, err := db.Collection(buildloggerCollection).InsertOne(ctx, log1)
	require.NoError(t, err)
	_, err = db.Collection(buildloggerCollection).InsertOne(ctx, log2)
	require.NoError(t, err)

	timeRange := TimeRange{
		StartAt: log2.Artifact.Chunks[0].Start,
		EndAt:   log2.Artifact.Chunks[len(log2.Artifact.Chunks)-1].End,
	}

	t.Run("NoEnv", func(t *testing.T) {
		l := Log{
			ID:        log1.ID,
			populated: true,
		}
		it, err := l.Download(ctx, timeRange)
		assert.Error(t, err)
		assert.Nil(t, it)
	})
	t.Run("NoConfig", func(t *testing.T) {
		l := Log{
			ID:        "DNE",
			populated: true,
		}
		l.Setup(env)
		it, err := l.Download(ctx, timeRange)
		assert.Error(t, err)
		assert.Nil(t, it)
	})
	conf := &CedarConfig{
		populated: true,
		Bucket:    BucketConfig{BuildLogsBucket: "."},
	}
	conf.Setup(env)
	require.NoError(t, conf.Save())
	t.Run("NoArtifact", func(t *testing.T) {
		l := Log{
			ID:        log1.ID,
			populated: true,
		}
		l.Setup(env)
		it, err := l.Download(ctx, timeRange)
		assert.Error(t, err)
		assert.Nil(t, it)
	})
	t.Run("WithID", func(t *testing.T) {
		log2.populated = true
		log2.Setup(env)
		it, err := log2.Download(ctx, timeRange)
		require.NoError(t, err)
		require.NotNil(t, it)

		expectedBucket, err := log2.Artifact.Type.Create(
			ctx,
			log2.env,
			conf.Bucket.BuildLogsBucket,
			log2.Artifact.Prefix,
			string(pail.S3PermissionsPrivate),
			false,
		)
		require.NoError(t, err)

		rawIt, ok := it.(*batchedIterator)
		require.True(t, ok)
		assert.Equal(t, expectedBucket, rawIt.bucket)
		assert.Equal(t, log2.Artifact.Chunks, rawIt.chunks)
		assert.Equal(t, timeRange, rawIt.timeRange)
		assert.Equal(t, 2, rawIt.batchSize)
		assert.False(t, rawIt.reverse)
		assert.NoError(t, it.Err())
		assert.NoError(t, it.Close())
	})
	t.Run("WithoutID", func(t *testing.T) {
		log2.ID = ""
		log2.populated = true
		log2.Setup(env)
		it, err := log2.Download(ctx, timeRange)
		require.NoError(t, err)
		require.NotNil(t, it)

		expectedBucket, err := log2.Artifact.Type.Create(
			ctx,
			log2.env,
			conf.Bucket.BuildLogsBucket,
			log2.Artifact.Prefix,
			string(pail.S3PermissionsPrivate),
			false,
		)
		require.NoError(t, err)

		rawIt, ok := it.(*batchedIterator)
		require.True(t, ok)
		assert.Equal(t, expectedBucket, rawIt.bucket)
		assert.Equal(t, log2.Artifact.Chunks, rawIt.chunks)
		assert.Equal(t, timeRange, rawIt.timeRange)
		assert.Equal(t, 2, rawIt.batchSize)
		assert.False(t, rawIt.reverse)
		assert.NoError(t, it.Err())
		assert.NoError(t, it.Close())
	})
}

func TestBuildloggerClose(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := env.Context()
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(buildloggerCollection).Drop(ctx))
	}()
	log1, log2 := getTestLogs(time.Now())

	_, err := db.Collection(buildloggerCollection).InsertOne(ctx, log1)
	require.NoError(t, err)
	_, err = db.Collection(buildloggerCollection).InsertOne(ctx, log2)
	require.NoError(t, err)

	t.Run("NoEnv", func(t *testing.T) {
		l := &Log{ID: log1.ID, populated: true}
		assert.Error(t, l.Close(ctx, 0))
	})
	t.Run("DNE", func(t *testing.T) {
		l := &Log{ID: "DNE"}
		l.Setup(env)
		assert.Error(t, l.Close(ctx, 0))
	})
	t.Run("WithID", func(t *testing.T) {
		e1 := 0
		l1 := &Log{ID: log1.ID, populated: true}
		l1.Setup(env)
		require.NoError(t, l1.Close(ctx, e1))

		updatedLog := &Log{}
		require.NoError(t, db.Collection(buildloggerCollection).FindOne(ctx, bson.M{"_id": log1.ID}).Decode(updatedLog))
		assert.Equal(t, log1.ID, updatedLog.ID)
		assert.Equal(t, log1.ID, updatedLog.Info.ID())
		assert.Equal(t, log1.CreatedAt.UTC().Round(time.Second), updatedLog.CreatedAt.Round(time.Second))
		assert.True(t, time.Since(updatedLog.CompletedAt) <= time.Second)
		assert.Equal(t, e1, updatedLog.Info.ExitCode)
		assert.Equal(t, log1.Info.Mainline, updatedLog.Info.Mainline)
		assert.Equal(t, log1.Info.Schema, updatedLog.Info.Schema)
		assert.Equal(t, log1.Artifact, updatedLog.Artifact)
	})
	t.Run("WithoutID", func(t *testing.T) {
		e2 := 9
		l2 := &Log{Info: log2.Info, populated: true}
		l2.Setup(env)
		require.NoError(t, l2.Close(ctx, e2))

		updatedLog := &Log{}
		require.NoError(t, db.Collection(buildloggerCollection).FindOne(ctx, bson.M{"_id": log2.ID}).Decode(updatedLog))
		assert.Equal(t, log2.ID, updatedLog.ID)
		assert.Equal(t, log2.ID, updatedLog.Info.ID())
		assert.Equal(t, log2.CreatedAt.UTC().Round(time.Second), updatedLog.CreatedAt.Round(time.Second))
		assert.True(t, time.Since(updatedLog.CompletedAt) <= time.Second)
		assert.Equal(t, e2, updatedLog.Info.ExitCode)
		assert.Equal(t, log2.Info.Mainline, updatedLog.Info.Mainline)
		assert.Equal(t, log2.Info.Schema, updatedLog.Info.Schema)
		assert.Equal(t, log2.Artifact, updatedLog.Artifact)
	})
}

func TestBuildloggerFindLogs(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := env.Context()
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(buildloggerCollection).Drop(ctx))
	}()
	log1, log2 := getTestLogs(time.Now())
	log1.Info.Tags = []string{"group", "tag1", "tag2"}
	log1.ID = log1.Info.ID()
	log2.Info.Tags = []string{"group", "tag1"}
	log2.ID = log2.Info.ID()
	log3, log4 := getTestLogs(time.Now().Add(time.Minute))
	log3.Info.TestName = "test2"
	log3.ID = log3.Info.ID()
	log4.Info.Tags = []string{"tag1"}
	log4.ID = log4.Info.ID()
	log5, log6 := getTestLogs(time.Now().Add(-24 * time.Hour))
	log5.Info.Execution = 1
	log5.ID = log5.Info.ID()
	log6.Info.Execution = 1
	log6.Info.Tags = []string{"group"}
	log6.ID = log6.Info.ID()
	log7, log8 := getTestLogs(time.Now().Add(3 * time.Minute))
	log7.Info.TestName = "test2"
	log7.Info.Execution = 1
	log7.ID = log7.Info.ID()
	log8.Info.TestName = ""
	log8.Info.Execution = 1
	log8.Info.Tags = []string{"group"}
	log8.ID = log8.Info.ID()

	_, err := db.Collection(buildloggerCollection).InsertMany(ctx, []interface{}{log1, log2, log3, log4, log5, log6, log7, log8})
	require.NoError(t, err)

	t.Run("NoEnv", func(t *testing.T) {
		logs := Logs{}
		opts := LogFindOptions{
			TimeRange: TimeRange{
				StartAt: time.Now().Add(-time.Hour),
				EndAt:   time.Now(),
			},
			Info: LogInfo{Project: log1.Info.Project},
		}
		assert.Error(t, logs.Find(ctx, opts))
		assert.Empty(t, logs.Logs)
		assert.False(t, logs.populated)
	})
	t.Run("InvalidTimeRange", func(t *testing.T) {
		logs := Logs{}
		logs.Setup(env)
		opts := LogFindOptions{
			TimeRange: TimeRange{},
			Info:      LogInfo{Project: log1.Info.Project},
		}
		assert.Error(t, logs.Find(ctx, opts))

		opts.TimeRange = TimeRange{
			StartAt: time.Now(),
			EndAt:   time.Now().Add(-time.Hour),
		}
		assert.Error(t, logs.Find(ctx, opts))
		assert.Empty(t, logs.Logs)
		assert.False(t, logs.populated)
	})
	t.Run("DNE", func(t *testing.T) {
		logs := Logs{}
		logs.Setup(env)
		opts := LogFindOptions{
			TimeRange: TimeRange{
				StartAt: time.Now().Add(-48 * time.Hour),
				EndAt:   time.Now(),
			},
			Info: LogInfo{Project: "DNE"},
		}
		assert.Equal(t, mongo.ErrNoDocuments, logs.Find(ctx, opts))
		assert.Empty(t, logs.Logs)
		assert.False(t, logs.populated)
	})
	t.Run("WithoutLimit", func(t *testing.T) {
		logs := Logs{}
		logs.Setup(env)
		opts := LogFindOptions{
			TimeRange: TimeRange{
				StartAt: time.Now().Add(-48 * time.Hour),
				EndAt:   time.Now(),
			},
			Info: LogInfo{Project: log1.Info.Project},
		}
		require.NoError(t, logs.Find(ctx, opts))
		require.Len(t, logs.Logs, 4)
		assert.Equal(t, log4.ID, logs.Logs[0].ID)
		assert.Equal(t, log2.ID, logs.Logs[1].ID)
		assert.Equal(t, log3.ID, logs.Logs[2].ID)
		assert.Equal(t, log1.ID, logs.Logs[3].ID)
		assert.True(t, logs.populated)
		assert.Equal(t, opts.TimeRange, logs.timeRange)
	})
	t.Run("WithLimit", func(t *testing.T) {
		logs := Logs{}
		logs.Setup(env)
		opts := LogFindOptions{
			TimeRange: TimeRange{
				StartAt: time.Now().Add(-78 * time.Hour),
				EndAt:   time.Now(),
			},
			Info:  LogInfo{Project: log1.Info.Project},
			Limit: 2,
		}
		require.NoError(t, logs.Find(ctx, opts))
		require.Len(t, logs.Logs, 2)
		assert.Equal(t, log4.ID, logs.Logs[0].ID)
		assert.Equal(t, log2.ID, logs.Logs[1].ID)
		assert.True(t, logs.populated)
		assert.Equal(t, opts.TimeRange, logs.timeRange)
	})
	t.Run("LatestExecution", func(t *testing.T) {
		logs := Logs{}
		logs.Setup(env)
		opts := LogFindOptions{
			TimeRange: TimeRange{
				StartAt: time.Now().Add(-78 * time.Hour),
				EndAt:   time.Now(),
			},
			Info:            LogInfo{TaskID: log1.Info.TaskID},
			LatestExecution: true,
		}
		require.NoError(t, logs.Find(ctx, opts))
		require.Len(t, logs.Logs, 2)
		assert.Equal(t, log7.ID, logs.Logs[0].ID)
		assert.Equal(t, log5.ID, logs.Logs[1].ID)
		assert.True(t, logs.populated)
		assert.Equal(t, opts.TimeRange, logs.timeRange)
	})
	t.Run("GroupNoTags", func(t *testing.T) {
		logs := Logs{}
		logs.Setup(env)
		opts := LogFindOptions{
			TimeRange: TimeRange{
				StartAt: time.Now().Add(-48 * time.Hour),
				EndAt:   time.Now(),
			},
			Info:  LogInfo{TaskID: log1.Info.TaskID},
			Group: "group",
		}
		require.NoError(t, logs.Find(ctx, opts))
		require.Len(t, logs.Logs, 1)
		assert.Equal(t, log1.ID, logs.Logs[0].ID)
		assert.True(t, logs.populated)
		assert.Equal(t, opts.TimeRange, logs.timeRange)
	})
	t.Run("GroupWithOverlappingTags", func(t *testing.T) {
		logs := Logs{}
		logs.Setup(env)
		opts := LogFindOptions{
			TimeRange: TimeRange{
				StartAt: time.Now().Add(-48 * time.Hour),
				EndAt:   time.Now(),
			},
			Info: LogInfo{
				TaskID: log2.Info.TaskID,
				Tags:   []string{"tag1", "tag2"},
			},
			Group: "group",
		}
		require.NoError(t, logs.Find(ctx, opts))
		require.Len(t, logs.Logs, 1)
		assert.Equal(t, log2.ID, logs.Logs[0].ID)
		assert.True(t, logs.populated)
		assert.Equal(t, opts.TimeRange, logs.timeRange)
	})
	t.Run("GroupDisjointTimes", func(t *testing.T) {
		logs := Logs{}
		logs.Setup(env)
		opts := LogFindOptions{
			TimeRange: TimeRange{
				StartAt: time.Now().Add(-78 * time.Hour),
				EndAt:   time.Now(),
			},
			Info: LogInfo{
				TaskID: log6.Info.TaskID,
			},
			Group:           "group",
			LatestExecution: true,
		}
		require.NoError(t, logs.Find(ctx, opts))
		require.Len(t, logs.Logs, 2)
		assert.Equal(t, log8.ID, logs.Logs[0].ID)
		assert.Equal(t, log6.ID, logs.Logs[1].ID)
		assert.True(t, logs.populated)
		assert.Equal(t, opts.TimeRange, logs.timeRange)
	})
	t.Run("OutsideOfTimeRange", func(t *testing.T) {
		logs := Logs{}
		logs.Setup(env)
		opts := LogFindOptions{
			TimeRange: TimeRange{
				StartAt: time.Now().Add(-48 * time.Hour),
				EndAt:   time.Now().Add(-25 * time.Hour),
			},
			Info: LogInfo{TestName: log1.Info.TestName},
		}
		require.Equal(t, mongo.ErrNoDocuments, logs.Find(ctx, opts))

		opts.TimeRange = TimeRange{
			StartAt: time.Now().Add(-22 * time.Hour),
			EndAt:   time.Now(),
		}
		require.Equal(t, mongo.ErrNoDocuments, logs.Find(ctx, opts))
	})
}

func TestBuildloggerCreateFindQuery(t *testing.T) {
	opts := LogFindOptions{
		TimeRange: TimeRange{
			StartAt: time.Now().Add(-time.Hour),
			EndAt:   time.Now(),
		},
		Info: LogInfo{
			Project:     "test",
			Version:     "0",
			Variant:     "linux",
			TaskName:    "task0",
			TaskID:      "task1",
			Execution:   1,
			TestName:    "test0",
			Trial:       3,
			ProcessName: "mongod0",
			Format:      LogFormatText,
			Tags:        []string{"tag1", "tag2"},
			Arguments:   map[string]string{"arg1": "val1", "arg2": "val2"},
			ExitCode:    1,
			Mainline:    true,
		},
	}

	t.Run("WithTaskID", func(t *testing.T) {
		search := createFindQuery(opts)
		_, ok := search[bsonutil.GetDottedKeyName(logInfoKey, logInfoMainlineKey)]
		assert.False(t, ok)
		assert.Equal(t, opts.Info.Project, search[bsonutil.GetDottedKeyName(logInfoKey, logInfoProjectKey)])
		assert.Equal(t, opts.Info.Version, search[bsonutil.GetDottedKeyName(logInfoKey, logInfoVersionKey)])
		assert.Equal(t, opts.Info.Variant, search[bsonutil.GetDottedKeyName(logInfoKey, logInfoVariantKey)])
		assert.Equal(t, opts.Info.TaskName, search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTaskNameKey)])
		assert.Equal(t, opts.Info.TaskID, search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTaskIDKey)])
		assert.Equal(t, opts.Info.Execution, search[bsonutil.GetDottedKeyName(logInfoKey, logInfoExecutionKey)])
		assert.Equal(t, opts.Info.TestName, search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTestNameKey)])
		assert.Equal(t, opts.Info.Trial, search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTrialKey)])
		assert.Equal(t, opts.Info.ProcessName, search[bsonutil.GetDottedKeyName(logInfoKey, logInfoProcessNameKey)])
		assert.Equal(t, opts.Info.Format, search[bsonutil.GetDottedKeyName(logInfoKey, logInfoFormatKey)])
		assert.Equal(t, bson.M{"$in": opts.Info.Tags}, search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTagsKey)])
		query, ok := search[bsonutil.GetDottedKeyName(logInfoKey, logInfoArgumentsKey)]
		require.True(t, ok)
		args := query.(bson.M)["$in"].([]bson.M)
		for _, arg := range args {
			assert.Len(t, arg, 1)
			for key, val := range arg {
				assert.Equal(t, opts.Info.Arguments[key], val)
			}
		}
		assert.Len(t, args, len(opts.Info.Arguments))
	})
	t.Run("WithoutTaskID", func(t *testing.T) {
		opts.Info.TaskID = ""
		search := createFindQuery(opts)
		assert.Equal(t, search[logCreatedAtKey], bson.M{"$lte": opts.TimeRange.EndAt})
		assert.Equal(t, search[logCompletedAtKey], bson.M{"$gte": opts.TimeRange.StartAt})
		_, ok := search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTaskIDKey)]
		assert.False(t, ok)
		assert.Equal(t, true, search[bsonutil.GetDottedKeyName(logInfoKey, logInfoMainlineKey)])
	})
	t.Run("OmitFields", func(t *testing.T) {
		opts.EmptyTestName = true
		opts.LatestExecution = true
		search := createFindQuery(opts)
		assert.Equal(t, opts.Info.Project, search[bsonutil.GetDottedKeyName(logInfoKey, logInfoProjectKey)])
		assert.Equal(t, opts.Info.Version, search[bsonutil.GetDottedKeyName(logInfoKey, logInfoVersionKey)])
		assert.Equal(t, opts.Info.Variant, search[bsonutil.GetDottedKeyName(logInfoKey, logInfoVariantKey)])
		assert.Equal(t, opts.Info.TaskName, search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTaskNameKey)])
		assert.Equal(t, nil, search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTestNameKey)])
		assert.Equal(t, opts.Info.Trial, search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTrialKey)])
		assert.Equal(t, opts.Info.ProcessName, search[bsonutil.GetDottedKeyName(logInfoKey, logInfoProcessNameKey)])
		assert.Equal(t, opts.Info.Format, search[bsonutil.GetDottedKeyName(logInfoKey, logInfoFormatKey)])
		assert.Equal(t, bson.M{"$in": opts.Info.Tags}, search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTagsKey)])
		query, ok := search[bsonutil.GetDottedKeyName(logInfoKey, logInfoArgumentsKey)]
		require.True(t, ok)
		args := query.(bson.M)["$in"].([]bson.M)
		for _, arg := range args {
			assert.Len(t, arg, 1)
			for key, val := range arg {
				assert.Equal(t, opts.Info.Arguments[key], val)
			}
		}
		assert.Len(t, args, len(opts.Info.Arguments))

		_, ok = search[bsonutil.GetDottedKeyName(logInfoKey, logInfoExecutionKey)]
		assert.False(t, ok)
	})
	t.Run("EmptyInfo", func(t *testing.T) {
		opts.Info = LogInfo{}
		opts.EmptyTestName = false
		opts.LatestExecution = false
		search := createFindQuery(opts)
		assert.Equal(t, search[logCreatedAtKey], bson.M{"$lte": opts.TimeRange.EndAt})
		assert.Equal(t, search[logCompletedAtKey], bson.M{"$gte": opts.TimeRange.StartAt})
		assert.Equal(t, true, search[bsonutil.GetDottedKeyName(logInfoKey, logInfoMainlineKey)])
		assert.Equal(t, 0, search[bsonutil.GetDottedKeyName(logInfoKey, logInfoExecutionKey)])
		assert.Len(t, search, 4)
	})
}

func TestBuildloggerMerge(t *testing.T) {
	env := cedar.GetEnvironment()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := env.GetDB()
	defer func() {
		assert.NoError(t, db.Collection(buildloggerCollection).Drop(ctx))
		assert.NoError(t, db.Collection(configurationCollection).Drop(ctx))
	}()
	log1, log2 := getTestLogs(time.Now())

	_, err := db.Collection(buildloggerCollection).InsertOne(ctx, log1)
	require.NoError(t, err)
	_, err = db.Collection(buildloggerCollection).InsertOne(ctx, log2)
	require.NoError(t, err)

	timeRange := TimeRange{
		StartAt: log2.Artifact.Chunks[0].Start,
		EndAt:   log2.Artifact.Chunks[len(log2.Artifact.Chunks)-1].End,
	}

	t.Run("NoEnv", func(t *testing.T) {
		logs := Logs{
			Logs:      []Log{*log1, *log2},
			populated: true,
			timeRange: timeRange,
		}
		it, err := logs.Merge(ctx)
		assert.Error(t, err)
		assert.Nil(t, it)
	})
	t.Run("Unpopulated", func(t *testing.T) {
		logs := Logs{
			Logs:      []Log{*log1, *log2},
			timeRange: timeRange,
		}
		logs.Setup(env)
		it, err := logs.Merge(ctx)
		assert.Error(t, err)
		assert.Nil(t, it)
	})
	t.Run("NoConfig", func(t *testing.T) {
		logs := Logs{
			Logs:      []Log{*log1, *log2},
			timeRange: timeRange,
			populated: true,
		}
		logs.Setup(env)
		it, err := logs.Merge(ctx)
		assert.Error(t, err)
		assert.Nil(t, it)
	})
	conf := &CedarConfig{
		populated: true,
		Bucket:    BucketConfig{BuildLogsBucket: "."},
	}
	conf.Setup(env)
	require.NoError(t, conf.Save())
	t.Run("MergeIterator", func(t *testing.T) {
		logs := Logs{
			Logs:      []Log{*log1, *log2},
			timeRange: timeRange,
			populated: true,
		}
		log1.Setup(env)
		it1, err := log1.Download(ctx, timeRange)
		require.NoError(t, err)
		require.NotNil(t, it1)
		log2.Setup(env)
		it2, err := log2.Download(ctx, timeRange)
		require.NoError(t, err)
		require.NotNil(t, it2)
		logs.Setup(env)
		it, err := logs.Merge(ctx)
		require.NoError(t, err)
		require.NotNil(t, it)

		assert.Equal(t, NewMergingIterator(it1, it2), it)
		assert.NoError(t, it.Close())
	})
}

// TODO: Remove this test once the task log migration is complete (EVG-13831).
func TestFindAndUpdateOutdatedTaskLogs(t *testing.T) {
	env := cedar.GetEnvironment()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := env.GetDB()
	defer func() {
		assert.NoError(t, db.Collection(buildloggerCollection).Drop(ctx))
	}()

	logs := []interface{}{
		&Log{
			ID: "one",
			Info: LogInfo{
				Project:     AgentLog,
				ProcessName: AgentLog,
				Tags:        []string{"tag1", "tag2", "tag3"},
			},
		},
		&Log{
			ID: "two",
			Info: LogInfo{
				Project:     AgentLog,
				ProcessName: AgentLog,
			},
		},
		&Log{
			ID: "three",
			Info: LogInfo{
				Project:     AgentLog,
				ProcessName: AgentLog,
				Tags:        []string{"tag1"},
			},
		},
		&Log{
			ID: "four",
			Info: LogInfo{
				Project:     TaskLog,
				ProcessName: TaskLog,
				Tags:        []string{"tag1", "tag2", "tag3"},
			},
		},
		&Log{
			ID: "five",
			Info: LogInfo{
				Project:     TaskLog,
				ProcessName: TaskLog,
			},
		},
		&Log{
			ID: "six",
			Info: LogInfo{
				Project:     TaskLog,
				ProcessName: TaskLog,
				Tags:        []string{"tag1"},
			},
		},
		&Log{
			ID: "seven",
			Info: LogInfo{
				Project:     SystemLog,
				ProcessName: SystemLog,
				Tags:        []string{"tag1", "tag2", "tag3"},
			},
		},
		&Log{
			ID: "eight",
			Info: LogInfo{
				Project:     SystemLog,
				ProcessName: SystemLog,
			},
		},
		&Log{
			ID: "nine",
			Info: LogInfo{
				Project:     SystemLog,
				ProcessName: SystemLog,
				Tags:        []string{"tag1"},
			},
		},
	}

	t.Run("NoEnv", func(t *testing.T) {
		assert.Error(t, FindAndUpdateOutdatedTaskLogs(ctx, nil, 1000))
	})
	t.Run("BigLimit", func(t *testing.T) {
		assert.NoError(t, db.Collection(buildloggerCollection).Drop(ctx))
		_, err := db.Collection(buildloggerCollection).InsertMany(ctx, logs)
		require.NoError(t, err)

		require.NoError(t, FindAndUpdateOutdatedTaskLogs(ctx, env, 1000))
		var updatedLogs []Log
		it, err := db.Collection(buildloggerCollection).Find(ctx, bson.M{})
		require.NoError(t, err)
		defer func() {
			assert.NoError(t, it.Close(ctx))
		}()
		require.NoError(t, it.All(ctx, &updatedLogs))
		require.Len(t, updatedLogs, 9)

		for _, log := range updatedLogs {
			assert.True(t, utility.StringSliceContains(log.Info.Tags, log.Info.Project))
			assert.Empty(t, log.Info.ProcessName)
		}
	})
	t.Run("SmallLimit", func(t *testing.T) {
		assert.NoError(t, db.Collection(buildloggerCollection).Drop(ctx))
		_, err := db.Collection(buildloggerCollection).InsertMany(ctx, logs)
		require.NoError(t, err)

		require.NoError(t, FindAndUpdateOutdatedTaskLogs(ctx, env, 3))
		var updatedLogs []Log
		it, err := db.Collection(buildloggerCollection).Find(ctx, bson.M{})
		require.NoError(t, err)
		defer func() {
			assert.NoError(t, it.Close(ctx))
		}()
		require.NoError(t, it.All(ctx, &updatedLogs))
		require.Len(t, updatedLogs, 9)

		updatedCount := 0
		outdatedCount := 0
		for _, log := range updatedLogs {
			if utility.StringSliceContains(log.Info.Tags, log.Info.Project) && log.Info.ProcessName == "" {
				updatedCount += 1
			}
			if !utility.StringSliceContains(log.Info.Tags, log.Info.ProcessName) && log.Info.ProcessName != "" {
				outdatedCount += 1
			}
		}
		assert.Equal(t, 3, updatedCount)
		assert.Equal(t, 6, outdatedCount)
	})
}

func getTestLogs(start time.Time) (*Log, *Log) {
	log1 := &Log{
		Info: LogInfo{
			Project:  "project",
			TaskID:   "task1",
			TestName: "test1",
			Mainline: true,
		},
		CreatedAt:   start.Add(-24 * time.Hour),
		CompletedAt: start.Add(-23 * time.Hour),
		Artifact: LogArtifactInfo{
			Type:    PailLocal,
			Prefix:  "log1",
			Version: 1,
		},
	}
	log1.ID = log1.Info.ID()
	log2 := &Log{
		Info: LogInfo{
			Project:  "project",
			TaskID:   "task2",
			TestName: "test2",
			Mainline: true,
		},
		CreatedAt:   start.Add(-2 * time.Hour),
		CompletedAt: start.Add(-time.Hour),
		Artifact: LogArtifactInfo{
			Type:    PailLocal,
			Prefix:  "log1",
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
