package model

import (
	"context"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/util"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/grip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestPerfFind(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(perfResultCollection).Drop(ctx))
	}()
	result1, result2 := getTestPerformanceResults()
	result1.Rollups.id = result1.ID
	result2.Rollups.id = result2.ID

	_, err := db.Collection(perfResultCollection).InsertOne(ctx, result1)
	require.NoError(t, err)
	_, err = db.Collection(perfResultCollection).InsertOne(ctx, result2)
	require.NoError(t, err)

	t.Run("DNE", func(t *testing.T) {
		r := PerformanceResult{ID: "DNE"}
		r.Setup(env)
		assert.Error(t, r.Find(ctx))
	})
	t.Run("NoEnv", func(t *testing.T) {
		r := PerformanceResult{ID: result1.ID}
		assert.Error(t, r.Find(ctx))
	})
	t.Run("WithID", func(t *testing.T) {
		r := PerformanceResult{ID: result1.ID}
		r.Setup(env)
		require.NoError(t, r.Find(ctx))
		assert.Equal(t, result1.ID, r.ID)
		assert.Equal(t, result1.Info, r.Info)
		assert.Equal(t, result1.Artifacts, r.Artifacts)
		assert.Equal(t, result1.Rollups, r.Rollups)
		assert.True(t, r.populated)
	})
	t.Run("WithoutID", func(t *testing.T) {
		r := PerformanceResult{Info: result2.Info}
		r.Setup(env)
		require.NoError(t, r.Find(ctx))
		assert.Equal(t, result2.ID, r.ID)
		assert.Equal(t, result2.Info, r.Info)
		assert.Equal(t, result2.Artifacts, r.Artifacts)
		assert.Equal(t, result2.Rollups, r.Rollups)
		assert.True(t, r.populated)
	})
}

func TestPerfSaveNew(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(perfResultCollection).Drop(ctx))
	}()
	result1, result2 := getTestPerformanceResults()

	t.Run("NoEnv", func(t *testing.T) {
		r := PerformanceResult{
			ID:        result1.ID,
			Info:      result1.Info,
			Artifacts: result1.Artifacts,
			Rollups:   result1.Rollups,
			populated: true,
		}
		assert.Error(t, r.SaveNew(ctx))
	})
	t.Run("Unpopulated", func(t *testing.T) {
		r := PerformanceResult{
			ID:        result1.ID,
			Info:      result1.Info,
			Artifacts: result1.Artifacts,
			Rollups:   result1.Rollups,
			populated: false,
		}
		r.Setup(env)
		assert.Error(t, r.SaveNew(ctx))
	})
	t.Run("WithID", func(t *testing.T) {
		savedResult := &PerformanceResult{}
		require.Error(t, db.Collection(perfResultCollection).FindOne(ctx, bson.M{"_id": result1.ID}).Decode(savedResult))

		r := PerformanceResult{
			ID:        result1.ID,
			Info:      result1.Info,
			Artifacts: result1.Artifacts,
			Rollups:   result1.Rollups,
			populated: true,
		}
		r.Setup(env)
		require.NoError(t, r.SaveNew(ctx))
		require.NoError(t, db.Collection(perfResultCollection).FindOne(ctx, bson.M{"_id": result1.ID}).Decode(savedResult))
		assert.Equal(t, result1.ID, savedResult.ID)
		assert.Equal(t, result1.Info, savedResult.Info)
		assert.Equal(t, result1.Artifacts, savedResult.Artifacts)
		assert.Equal(t, result1.Rollups, savedResult.Rollups)
	})
	t.Run("WithoutID", func(t *testing.T) {
		savedResult := &PerformanceResult{}
		require.Error(t, db.Collection(perfResultCollection).FindOne(ctx, bson.M{"_id": result2.ID}).Decode(savedResult))

		r := PerformanceResult{
			Info:      result2.Info,
			Artifacts: result2.Artifacts,
			Rollups:   result2.Rollups,
			populated: true,
		}
		r.Setup(env)
		require.NoError(t, r.SaveNew(ctx))
		require.NoError(t, db.Collection(perfResultCollection).FindOne(ctx, bson.M{"_id": result2.ID}).Decode(savedResult))
		assert.Equal(t, result2.ID, savedResult.ID)
		assert.Equal(t, result2.Info, savedResult.Info)
		assert.Equal(t, result2.Artifacts, savedResult.Artifacts)
		assert.Equal(t, result2.Rollups, savedResult.Rollups)
	})
	t.Run("AlreadyExists", func(t *testing.T) {
		_, err := db.Collection(perfResultCollection).ReplaceOne(ctx, bson.M{"_id": result2.ID}, result2, options.Replace().SetUpsert(true))
		require.NoError(t, err)

		r := PerformanceResult{
			ID:        result2.ID,
			populated: true,
		}
		r.Setup(env)
		require.Error(t, r.SaveNew(ctx))
	})
}

func TestPerfRemove(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(perfResultCollection).Drop(ctx))
	}()
	result1, result2 := getTestPerformanceResults()

	_, err := db.Collection(perfResultCollection).InsertOne(ctx, result1)
	require.NoError(t, err)
	_, err = db.Collection(perfResultCollection).InsertOne(ctx, result2)
	require.NoError(t, err)

	t.Run("NoEnv", func(t *testing.T) {
		r := PerformanceResult{ID: result1.ID}
		n, err := r.Remove(ctx)
		assert.Equal(t, -1, n)
		assert.Error(t, err)

		savedResult := &PerformanceResult{}
		require.NoError(t, db.Collection(perfResultCollection).FindOne(ctx, bson.M{"_id": result1.ID}).Decode(savedResult))
	})
	t.Run("DNE", func(t *testing.T) {
		r := PerformanceResult{ID: "DNE"}
		r.Setup(env)
		n, err := r.Remove(ctx)
		assert.Zero(t, n)
		require.NoError(t, err)
	})
	t.Run("WithID", func(t *testing.T) {
		r := PerformanceResult{ID: result1.ID}
		r.Setup(env)
		n, err := r.Remove(ctx)
		assert.Equal(t, 1, n)
		require.NoError(t, err)

		savedResult := &PerformanceResult{}
		require.Error(t, db.Collection(perfResultCollection).FindOne(ctx, bson.M{"_id": result1.ID}).Decode(savedResult))
	})
	t.Run("WithoutID", func(t *testing.T) {
		r := PerformanceResult{Info: result2.Info}
		r.Setup(env)
		n, err := r.Remove(ctx)
		assert.Equal(t, 1, n)
		require.NoError(t, err)

		savedResult := &PerformanceResult{}
		require.Error(t, db.Collection(perfResultCollection).FindOne(ctx, bson.M{"_id": result2.ID}).Decode(savedResult))
	})
}

func TestPerfAppendArtifacts(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(perfResultCollection).Drop(ctx))
	}()
	result1, result2 := getTestPerformanceResults()

	_, err := db.Collection(perfResultCollection).InsertOne(ctx, result1)
	require.NoError(t, err)
	_, err = db.Collection(perfResultCollection).InsertOne(ctx, result2)
	require.NoError(t, err)

	artifacts := []ArtifactInfo{
		{
			Type:        PailS3,
			Bucket:      "bucket",
			Prefix:      "prefix",
			Path:        "path/path",
			Format:      FileFTDC,
			Compression: FileUncompressed,
			Schema:      SchemaRawEvents,
			Tags:        []string{"tag1", "tag2"},
			CreatedAt:   time.Now().UTC().Truncate(time.Millisecond),
		},
		{
			Type:        PailLocal,
			Bucket:      "local_bucket",
			Prefix:      "local_prefix",
			Path:        "local_path",
			Format:      FileBSON,
			Compression: FileGz,
			Schema:      SchemaHistogram,
			Tags:        []string{"tag1", "tag2"},
			CreatedAt:   time.Now().Add(-24 * time.Hour).UTC().Truncate(time.Millisecond),
		},
	}

	t.Run("NoEnv", func(t *testing.T) {
		result := PerformanceResult{ID: result1.ID}
		assert.Error(t, result.AppendArtifacts(ctx, artifacts))

		savedResult := &PerformanceResult{}
		require.NoError(t, db.Collection(perfResultCollection).FindOne(ctx, bson.M{"_id": result1.ID}).Decode(savedResult))
		assert.NotEqual(t, append(result1.Artifacts, artifacts...), savedResult.Artifacts)
	})
	t.Run("DNE", func(t *testing.T) {
		result := PerformanceResult{ID: "DNE"}
		assert.Error(t, result.AppendArtifacts(ctx, artifacts))
	})
	t.Run("WithID", func(t *testing.T) {
		result := PerformanceResult{ID: result1.ID}
		result.Setup(env)
		assert.NoError(t, result.AppendArtifacts(ctx, artifacts))

		savedResult := &PerformanceResult{}
		require.NoError(t, db.Collection(perfResultCollection).FindOne(ctx, bson.M{"_id": result1.ID}).Decode(savedResult))
		assert.Equal(t, result1.ID, savedResult.ID)
		assert.Equal(t, result1.Info, savedResult.Info)
		assert.Equal(t, append(result1.Artifacts, artifacts...), savedResult.Artifacts)
		assert.Equal(t, result1.Rollups, savedResult.Rollups)
	})
	t.Run("WithoutID", func(t *testing.T) {
		result := PerformanceResult{ID: result2.ID}
		result.Setup(env)
		assert.NoError(t, result.AppendArtifacts(ctx, artifacts))

		savedResult := &PerformanceResult{}
		require.NoError(t, db.Collection(perfResultCollection).FindOne(ctx, bson.M{"_id": result2.ID}).Decode(savedResult))
		assert.Equal(t, result2.ID, savedResult.ID)
		assert.Equal(t, result2.Info, savedResult.Info)
		assert.Equal(t, append(result2.Artifacts, artifacts...), savedResult.Artifacts)
		assert.Equal(t, result2.Rollups, savedResult.Rollups)
	})
}

func TestPerfIncFailedRollupAttempts(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	lastCount := 0
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(perfResultCollection).Drop(ctx))
	}()
	result1, _ := getTestPerformanceResults()

	_, err := db.Collection(perfResultCollection).InsertOne(ctx, result1)
	require.NoError(t, err)

	t.Run("NoEnv", func(t *testing.T) {
		r := &PerformanceResult{ID: result1.ID}
		assert.Error(t, r.IncFailedRollupAttempts(ctx))

		savedResult := &PerformanceResult{}
		require.NoError(t, db.Collection(perfResultCollection).FindOne(ctx, bson.M{"_id": result1.ID}).Decode(savedResult))
		assert.Zero(t, savedResult.FailedRollupAttempts)
	})
	t.Run("DNE", func(t *testing.T) {
		r := &PerformanceResult{ID: "DNE"}
		r.Setup(env)
		assert.Error(t, r.IncFailedRollupAttempts(ctx))
	})
	t.Run("WithID", func(t *testing.T) {
		r := &PerformanceResult{ID: result1.ID}
		r.Setup(env)
		require.NoError(t, r.IncFailedRollupAttempts(ctx))

		updatedResult := &PerformanceResult{}
		require.NoError(t, db.Collection(perfResultCollection).FindOne(ctx, bson.M{"_id": result1.ID}).Decode(updatedResult))
		assert.Equal(t, result1.ID, updatedResult.ID)
		assert.Equal(t, result1.Info, updatedResult.Info)
		assert.Equal(t, result1.CreatedAt, updatedResult.CreatedAt)
		assert.Equal(t, result1.CompletedAt, updatedResult.CompletedAt)
		assert.Equal(t, result1.Artifacts, updatedResult.Artifacts)
		assert.Equal(t, lastCount+1, updatedResult.FailedRollupAttempts)
		assert.Equal(t, result1.Rollups, updatedResult.Rollups)
		lastCount = 1
	})
	t.Run("WithoutID", func(t *testing.T) {
		r := &PerformanceResult{Info: result1.Info}
		r.Setup(env)
		require.NoError(t, r.IncFailedRollupAttempts(ctx))

		updatedResult := &PerformanceResult{}
		require.NoError(t, db.Collection(perfResultCollection).FindOne(ctx, bson.M{"_id": result1.ID}).Decode(updatedResult))
		assert.Equal(t, result1.ID, updatedResult.ID)
		assert.Equal(t, result1.Info, updatedResult.Info)
		assert.Equal(t, result1.CreatedAt, updatedResult.CreatedAt)
		assert.Equal(t, result1.CompletedAt, updatedResult.CompletedAt)
		assert.Equal(t, result1.Artifacts, updatedResult.Artifacts)
		assert.Equal(t, lastCount+1, updatedResult.FailedRollupAttempts)
		assert.Equal(t, result1.Rollups, updatedResult.Rollups)
	})
}

func TestPerfClose(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(perfResultCollection).Drop(ctx))
	}()
	result1, result2 := getTestPerformanceResults()

	_, err := db.Collection(perfResultCollection).InsertOne(ctx, result1)
	require.NoError(t, err)
	_, err = db.Collection(perfResultCollection).InsertOne(ctx, result2)
	require.NoError(t, err)

	t.Run("NoEnv", func(t *testing.T) {
		ts := time.Now().UTC().Truncate(time.Millisecond)
		r := &PerformanceResult{ID: result1.ID}
		assert.Error(t, r.Close(ctx, ts))

		savedResult := &PerformanceResult{}
		require.NoError(t, db.Collection(perfResultCollection).FindOne(ctx, bson.M{"_id": result1.ID}).Decode(savedResult))
		assert.NotEqual(t, ts, savedResult.CompletedAt)
	})
	t.Run("DNE", func(t *testing.T) {
		r := &PerformanceResult{ID: "DNE"}
		r.Setup(env)
		assert.Error(t, r.Close(ctx, time.Now()))
	})
	t.Run("WithID", func(t *testing.T) {
		ts := time.Now().Add(-15 * time.Minute).UTC().Truncate(time.Millisecond)
		r := &PerformanceResult{ID: result1.ID}
		r.Setup(env)
		require.NoError(t, r.Close(ctx, ts))

		updatedResult := &PerformanceResult{}
		require.NoError(t, db.Collection(perfResultCollection).FindOne(ctx, bson.M{"_id": result1.ID}).Decode(updatedResult))
		assert.Equal(t, result1.ID, updatedResult.ID)
		assert.Equal(t, result1.Info, updatedResult.Info)
		assert.Equal(t, result1.CreatedAt, updatedResult.CreatedAt)
		assert.Equal(t, ts, updatedResult.CompletedAt)
		assert.Equal(t, result1.Artifacts, updatedResult.Artifacts)
		assert.Equal(t, result1.Rollups, updatedResult.Rollups)
	})
	t.Run("WithoutID", func(t *testing.T) {
		ts := time.Now().UTC().Truncate(time.Millisecond)
		r := &PerformanceResult{Info: result2.Info}
		r.Setup(env)
		require.NoError(t, r.Close(ctx, ts))

		updatedResult := &PerformanceResult{}
		require.NoError(t, db.Collection(perfResultCollection).FindOne(ctx, bson.M{"_id": result2.ID}).Decode(updatedResult))
		assert.Equal(t, result2.ID, updatedResult.ID)
		assert.Equal(t, result2.Info, updatedResult.Info)
		assert.Equal(t, result2.CreatedAt, updatedResult.CreatedAt)
		assert.Equal(t, ts, updatedResult.CompletedAt)
		assert.Equal(t, result2.Artifacts, updatedResult.Artifacts)
		assert.Equal(t, result2.Rollups, updatedResult.Rollups)
	})
}

func getTestPerformanceResults() (*PerformanceResult, *PerformanceResult) {
	result1 := &PerformanceResult{
		Info: PerformanceResultInfo{
			Project:   "project1",
			Version:   "version1",
			Order:     500,
			Variant:   "variant1",
			TaskName:  "task_name1",
			TaskID:    "task_id1",
			Execution: 1,
			TestName:  "test_name1",
			Trial:     1,
			Parent:    "parent",
			Tags:      []string{"tag1", "tag2", "tag3"},
			Arguments: map[string]int32{"threads": 64},
			Mainline:  true,
		},
		CreatedAt:   time.Now().Add(-24 * time.Hour).UTC().Truncate(time.Millisecond),
		CompletedAt: time.Now().Add(-23 * time.Hour).UTC().Truncate(time.Millisecond),
		Artifacts: []ArtifactInfo{
			{
				Type:        PailLocal,
				Bucket:      "bucket",
				Prefix:      "prefix",
				Path:        "path",
				Format:      FileFTDC,
				Compression: FileGz,
				Schema:      SchemaRawEvents,
				Tags:        []string{"artifacttag1", "artifacttag2"},
				CreatedAt:   time.Now().UTC().Truncate(time.Millisecond),
			},
		},
		Rollups: PerfRollups{
			Stats: []PerfRollupValue{
				{
					Name:          "latency",
					Value:         int32(1),
					Version:       1,
					MetricType:    MetricTypeLatency,
					UserSubmitted: true,
					Valid:         true,
				},
				{
					Name:       "mean",
					Value:      10.7,
					Version:    3,
					MetricType: MetricTypeMean,
				},
			},
			ProcessedAt: time.Now().Add(-24 * time.Hour).UTC().Truncate(time.Millisecond),
			Valid:       true,
		},
	}
	result1.ID = result1.Info.ID()
	result2 := &PerformanceResult{
		Info: PerformanceResultInfo{
			Project:  "project",
			TestName: "test2",
		},
		CreatedAt:   time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Millisecond),
		CompletedAt: time.Now().Add(-time.Hour).UTC().Truncate(time.Millisecond),
		Artifacts:   []ArtifactInfo{},
		Rollups:     PerfRollups{Stats: []PerfRollupValue{}},
	}
	result2.ID = result2.Info.ID()

	return result1, result2
}

type perfResultsSuite struct {
	ctx    context.Context
	cancel context.CancelFunc
	r      *PerformanceResults

	suite.Suite
}

func TestPerformanceResultsSuite(t *testing.T) {
	suite.Run(t, new(perfResultsSuite))
}

func getTimeForTestingByDate(day int) time.Time {
	return time.Date(2018, 10, day, 0, 0, 0, 0, time.Local)
}

func (s *perfResultsSuite) SetupTest() {
	env := cedar.GetEnvironment()
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.r = new(PerformanceResults)
	s.r.Setup(env)
	args := make(map[string]int32)
	args["timeout"] = 12
	info := PerformanceResultInfo{
		Parent:    "123",
		Version:   "1",
		Order:     1,
		TaskName:  "task",
		Project:   "test",
		Trial:     12,
		Tags:      []string{"tag1", "bson"},
		Arguments: args,
		Mainline:  true,
	}
	source := []ArtifactInfo{}
	result := CreatePerformanceResult(info, source, nil)
	result.Setup(cedar.GetEnvironment())
	result.CreatedAt = getTimeForTestingByDate(15)
	result.Version = 1
	s.True(result.CompletedAt.IsZero())
	s.NoError(result.SaveNew(s.ctx))

	info = PerformanceResultInfo{
		Parent:   "234",
		Version:  "1",
		Order:    2,
		TaskName: "task",
		Trial:    10,
		Tags:     []string{"tag2", "json"},
		Mainline: true,
	}
	result2 := CreatePerformanceResult(info, source, nil)
	result2.Setup(cedar.GetEnvironment())
	result2.CreatedAt = getTimeForTestingByDate(16)
	result2.CompletedAt = getTimeForTestingByDate(18)
	result2.Version = 2
	s.NoError(result2.SaveNew(s.ctx))

	info = PerformanceResultInfo{
		Version:  "1",
		Order:    3,
		TaskName: "task",
		Mainline: true,
	}
	result3 := CreatePerformanceResult(info, source, nil)
	result3.Setup(cedar.GetEnvironment())
	s.NoError(result3.SaveNew(s.ctx))
}

func (s *perfResultsSuite) TearDownTest() {
	defer s.cancel()
	s.NoError(s.r.env.GetDB().Collection(perfResultCollection).Drop(s.ctx))
}

func (s *perfResultsSuite) TestFindWithNilEnv() {
	env := s.r.env
	s.r.env = nil
	defer func() {
		s.r.env = env
	}()
	start := getTimeForTestingByDate(15)
	options := PerfFindOptions{
		Interval: util.GetTimeRange(start, time.Hour*48),
		MaxDepth: 5,
	}
	s.Error(s.r.Find(s.ctx, options))
}

func (s *perfResultsSuite) TestFindResultsByTimeInterval() {
	start := getTimeForTestingByDate(15)
	options := PerfFindOptions{
		Interval: util.GetTimeRange(start, time.Hour*48),
		MaxDepth: 5,
	}
	s.NoError(s.r.Find(s.ctx, options))
	s.Require().Len(s.r.Results, 1)
	s.Equal(1, s.r.Results[0].Version)

	start = getTimeForTestingByDate(16)
	options.Interval = util.GetTimeRange(start, time.Hour*48)
	s.NoError(s.r.Find(s.ctx, options))
	s.Require().Len(s.r.Results, 1)
	s.Equal(2, s.r.Results[0].Version)

	start = getTimeForTestingByDate(15)
	options.Interval = util.GetTimeRange(start, time.Hour*72)
	s.NoError(s.r.Find(s.ctx, options))
	s.Len(s.r.Results, 2)
	options.Limit = 1
	s.NoError(s.r.Find(s.ctx, options))
	s.Len(s.r.Results, 1)

	options.Interval = util.GetTimeRange(start, -time.Hour*24)
	s.Error(s.r.Find(s.ctx, options))
}

func (s *perfResultsSuite) TestFindResultsWithOptionsInfo() {
	start := getTimeForTestingByDate(15)
	options := PerfFindOptions{
		Interval: util.GetTimeRange(start, time.Hour*72),
		MaxDepth: 5,
	}
	options.Info.Version = "1"
	s.NoError(s.r.Find(s.ctx, options))
	s.Len(s.r.Results, 2)

	options.Info.Tags = []string{"tag1", "tag2", "tag3"}
	s.NoError(s.r.Find(s.ctx, options))
	s.Len(s.r.Results, 2)

	options.Info.Project = "test"
	s.NoError(s.r.Find(s.ctx, options))
	s.Require().Len(s.r.Results, 1)
	s.Equal("1", s.r.Results[0].Info.Version, "%+v", options)

	options.Info = PerformanceResultInfo{}
	options.Info.Trial = 10
	s.Len(s.r.Results, 1)
	options.Info = PerformanceResultInfo{}
	options.Info.Arguments = make(map[string]int32)
	options.Info.Arguments["timeout"] = 12
	options.Info.Arguments["something"] = 24

	s.NoError(s.r.Find(s.ctx, options))
	s.Require().Len(s.r.Results, 1)
	s.Equal(s.r.Results[0].Info.Version, "1")
}

func (s *perfResultsSuite) TestFindResultsWithSortAndLimit() {
	options := PerfFindOptions{
		Interval: util.TimeRange{
			StartAt: time.Time{},
			EndAt:   time.Now(),
		},
		Info: PerformanceResultInfo{TaskName: "task"},
		Sort: []string{bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoOrderKey)},
	}
	s.NoError(s.r.Find(s.ctx, options))
	s.Len(s.r.Results, 3)
	for i := 1; i < len(s.r.Results); i++ {
		s.True(s.r.Results[i-1].Info.Order >= s.r.Results[i].Info.Order)
	}

	options.Limit = 2
	s.NoError(s.r.Find(s.ctx, options))
	s.Len(s.r.Results, 2)
	for i := 1; i < len(s.r.Results); i++ {
		s.True(s.r.Results[i-1].Info.Order >= s.r.Results[i].Info.Order)
	}
}

func (s *perfResultsSuite) TestSearchResultsWithParent() {
	// nodeA -> nodeB and nodeC, nodeB -> nodeD
	s.r = new(PerformanceResults)
	s.r.Setup(cedar.GetEnvironment())

	info := PerformanceResultInfo{Parent: "NA"}
	source := []ArtifactInfo{}
	nodeA := CreatePerformanceResult(info, source, nil)
	nodeA.Setup(cedar.GetEnvironment())
	nodeA.CreatedAt = getTimeForTestingByDate(15)
	s.NoError(nodeA.SaveNew(s.ctx))

	info = PerformanceResultInfo{
		Parent: nodeA.ID,
		Tags:   []string{"tag0"},
	}
	nodeB := CreatePerformanceResult(info, []ArtifactInfo{}, nil)
	nodeB.Setup(cedar.GetEnvironment())
	nodeB.CreatedAt = getTimeForTestingByDate(16)
	s.NoError(nodeB.SaveNew(s.ctx))

	info.Version = "C"
	nodeC := CreatePerformanceResult(info, []ArtifactInfo{}, nil)
	nodeC.Setup(cedar.GetEnvironment())
	nodeC.CreatedAt = getTimeForTestingByDate(16)
	s.NoError(nodeC.SaveNew(s.ctx))

	info = PerformanceResultInfo{
		Parent: nodeB.ID,
		Tags:   []string{"tag1"},
	}
	nodeD := CreatePerformanceResult(info, []ArtifactInfo{}, nil)
	nodeD.Setup(cedar.GetEnvironment())
	nodeD.CreatedAt = getTimeForTestingByDate(17)
	s.NoError(nodeD.SaveNew(s.ctx))

	// Without $graphLookup
	options := PerfFindOptions{
		MaxDepth: 5,
	}
	options.Info.Parent = nodeA.ID
	s.NoError(s.r.Find(s.ctx, options))
	s.Require().Len(s.r.Results, 4)

	s.Equal(nodeA.ID, s.r.Results[0].ID)
	s.Equal(nodeB.ID, s.r.Results[2].ID)
	s.Equal(nodeC.ID, s.r.Results[3].ID)
	s.Equal(nodeD.ID, s.r.Results[1].ID)

	s.Equal("NA", s.r.Results[0].Info.Parent)
	s.Equal(nodeB.ID, s.r.Results[1].Info.Parent)
	s.Equal(nodeA.ID, s.r.Results[2].Info.Parent)
	s.Equal(nodeA.ID, s.r.Results[3].Info.Parent)

	// Test min through max depth without $graphLookup
	options = PerfFindOptions{
		MaxDepth: 0,
	}
	options.Info.Parent = nodeA.ID
	s.NoError(s.r.Find(s.ctx, options))
	s.Require().Len(s.r.Results, 1)
	s.Equal(s.r.Results[0].ID, nodeA.ID)

	options = PerfFindOptions{
		MaxDepth: 1,
	}
	options.Info.Parent = nodeA.ID
	s.NoError(s.r.Find(s.ctx, options))
	s.Require().Len(s.r.Results, 3)
	grip.Notice(s.r.Results)
	s.Equal(s.r.Results[0].ID, nodeA.ID)
	s.Equal(s.r.Results[1].Info.Parent, nodeA.ID)
	s.Equal(s.r.Results[2].Info.Parent, nodeA.ID)

	options = PerfFindOptions{
		MaxDepth: -1,
	}
	options.Info.Parent = nodeA.ID
	s.NoError(s.r.Find(s.ctx, options))
	s.Require().Len(s.r.Results, 4)

	s.Equal(nodeA.ID, s.r.Results[0].ID)
	s.Equal(nodeB.ID, s.r.Results[2].ID)
	s.Equal(nodeC.ID, s.r.Results[3].ID)
	s.Equal(nodeD.ID, s.r.Results[1].ID)

	s.Equal("NA", s.r.Results[0].Info.Parent)
	s.Equal(nodeB.ID, s.r.Results[1].Info.Parent)
	s.Equal(nodeA.ID, s.r.Results[2].Info.Parent)
	s.Equal(nodeA.ID, s.r.Results[3].Info.Parent)

	// With $graphLookup
	options = PerfFindOptions{
		MaxDepth:    5,
		GraphLookup: true,
	}
	options.Info.Parent = nodeA.ID
	s.NoError(s.r.Find(s.ctx, options))
	s.Require().Len(s.r.Results, 4)
	s.Equal(s.r.Results[0].ID, nodeA.ID)
	s.Equal(s.r.Results[1].ID, nodeD.ID)
	s.Equal(s.r.Results[2].Info.Parent, nodeA.ID)
	s.Equal(s.r.Results[3].Info.Parent, nodeA.ID)

	// Test min through max depth with $graphLookup
	options = PerfFindOptions{
		MaxDepth:    0,
		GraphLookup: true,
	}
	options.Info.Parent = nodeA.ID
	s.NoError(s.r.Find(s.ctx, options))
	s.Require().Len(s.r.Results, 1)
	s.Equal(s.r.Results[0].ID, nodeA.ID)

	options = PerfFindOptions{
		MaxDepth:    1,
		GraphLookup: true,
	}
	options.Info.Parent = nodeA.ID
	s.NoError(s.r.Find(s.ctx, options))
	s.Require().Len(s.r.Results, 3)
	s.Equal(s.r.Results[0].ID, nodeA.ID)
	s.Equal(s.r.Results[1].Info.Parent, nodeA.ID)
	s.Equal(s.r.Results[2].Info.Parent, nodeA.ID)

	options = PerfFindOptions{
		MaxDepth:    -1,
		GraphLookup: true,
	}
	options.Info.Parent = nodeA.ID
	s.NoError(s.r.Find(s.ctx, options))
	s.Require().Len(s.r.Results, 4)
	s.Equal(s.r.Results[0].ID, nodeA.ID)
	s.Equal(s.r.Results[1].ID, nodeD.ID)
	s.Equal(s.r.Results[2].Info.Parent, nodeA.ID)
	s.Equal(s.r.Results[3].Info.Parent, nodeA.ID)

	// Test tag filtering with $graphLookup
	options = PerfFindOptions{
		MaxDepth:    5,
		GraphLookup: true,
	}
	options.Info.Parent = nodeA.ID
	options.Info.Tags = []string{"tag1"}
	s.NoError(s.r.Find(s.ctx, options))
	s.Require().Len(s.r.Results, 2)
	s.Equal(s.r.Results[0].ID, nodeA.ID)
	s.Equal(s.r.Results[1].ID, nodeD.ID)

	// Test remove removes all children
	root := PerformanceResult{ID: nodeA.ID}
	root.Setup(cedar.GetEnvironment())
	numRemoved, err := root.Remove(s.ctx)
	s.NoError(err)
	s.Equal(4, numRemoved)
	options = PerfFindOptions{
		MaxDepth:    -1,
		GraphLookup: true,
	}
	options.Info.Parent = nodeA.ID
	s.NoError(s.r.Find(s.ctx, options))
	s.Len(s.r.Results, 0)
}

func (s *perfResultsSuite) TestFindOutdated() {
	s.r = new(PerformanceResults)
	s.r.Setup(cedar.GetEnvironment())
	source := []ArtifactInfo{
		{
			Format: FileFTDC,
		},
		{
			Format: FileBSON,
		},
	}
	rollupName := "SomeRollup"

	noFTDCData := PerformanceResultInfo{Project: "NoFTDCData"}
	result := CreatePerformanceResult(noFTDCData, source[1:], nil)
	result.CreatedAt = time.Now()
	result.Setup(cedar.GetEnvironment())
	s.Require().NoError(result.SaveNew(s.ctx))

	correctVersionValid := PerformanceResultInfo{Project: "CorrectVersionValid"}
	result = CreatePerformanceResult(correctVersionValid, source, nil)
	result.CreatedAt = time.Now()
	result.Rollups.Stats = append(
		result.Rollups.Stats,
		PerfRollupValue{
			Name:    rollupName,
			Value:   1,
			Version: 2,
			Valid:   true,
		},
	)
	result.Setup(cedar.GetEnvironment())
	s.Require().NoError(result.SaveNew(s.ctx))

	correctVersionInvalid := PerformanceResultInfo{Project: "CorrectVersionInvalid"}
	result = CreatePerformanceResult(correctVersionInvalid, source, nil)
	result.CreatedAt = time.Now()
	result.Rollups.Stats = append(
		result.Rollups.Stats,
		PerfRollupValue{
			Name:    rollupName,
			Version: 2,
			Valid:   false,
		},
	)
	result.Setup(cedar.GetEnvironment())
	s.Require().NoError(result.SaveNew(s.ctx))

	outdated := PerformanceResultInfo{Project: "Outdated"}
	result = CreatePerformanceResult(outdated, source, nil)
	result.CreatedAt = time.Now()
	result.FailedRollupAttempts = 2
	result.Rollups.Stats = append(
		result.Rollups.Stats,
		PerfRollupValue{
			Name:    rollupName,
			Value:   1.01,
			Version: 1,
			Valid:   true,
		},
	)
	result.Setup(cedar.GetEnvironment())
	s.Require().NoError(result.SaveNew(s.ctx))

	outdatedOld := PerformanceResultInfo{Project: "OutdatedOld"}
	result = CreatePerformanceResult(outdatedOld, source, nil)
	result.CreatedAt = time.Now().Add(-time.Hour)
	result.Rollups.Stats = append(
		result.Rollups.Stats,
		PerfRollupValue{
			Name:    rollupName,
			Value:   1.01,
			Version: 1,
			Valid:   true,
		},
	)
	result.Setup(cedar.GetEnvironment())
	s.Require().NoError(result.SaveNew(s.ctx))

	failedTooManyTimes := PerformanceResultInfo{Project: "FailedTooManyTimes"}
	result = CreatePerformanceResult(failedTooManyTimes, source, nil)
	result.CreatedAt = time.Now()
	result.FailedRollupAttempts = 3
	result.Setup(cedar.GetEnvironment())
	s.Require().NoError(result.SaveNew(s.ctx))

	s.Require().NoError(s.r.FindOutdatedRollups(s.ctx, rollupName, 2, time.Now().Add(-time.Hour), 3))
	s.Require().Len(s.r.Results, 1)
	s.Equal(outdated.ID(), s.r.Results[0].Info.ID())

	doesNotExist := PerformanceResultInfo{Project: "DNE"}
	result = CreatePerformanceResult(doesNotExist, source, nil)
	result.Rollups.Stats = append(
		result.Rollups.Stats,
		PerfRollupValue{
			Name:    "DNE",
			Value:   1.01,
			Version: 1,
			Valid:   true,
		},
	)
	result.Setup(cedar.GetEnvironment())
	s.Require().NoError(result.SaveNew(s.ctx))

	s.Require().NoError(s.r.FindOutdatedRollups(s.ctx, "DNE", 1, time.Now().Add(-2*time.Hour), 3))
	s.Require().Len(s.r.Results, 4)
	for _, result := range s.r.Results {
		s.NotEqual(doesNotExist.ID(), result.Info.ID())
	}

	// nil env
	env := s.r.env
	s.r.env = nil
	defer func() {
		s.r.env = env
	}()
	s.Error(s.r.FindOutdatedRollups(s.ctx, rollupName, 2, time.Now().Add(-time.Hour), 3))
}
