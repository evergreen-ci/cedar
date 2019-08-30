package model

import (
	"context"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson"
)

type perfRollupSuite struct {
	ctx    context.Context
	cancel context.CancelFunc
	r      *PerfRollups
	suite.Suite
}

func TestPerfRollupSuite(t *testing.T) {
	suite.Run(t, new(perfRollupSuite))
}

func (s *perfRollupSuite) SetupTest() {
	s.r = new(PerfRollups)
	s.r.Stats = []PerfRollupValue{}
	s.r.Setup(cedar.GetEnvironment())
	s.r.id = "123"

	s.ctx, s.cancel = context.WithCancel(context.Background())

	_, err := s.r.env.GetDB().Collection(perfResultCollection).InsertOne(
		s.ctx,
		PerformanceResult{ID: s.r.id, Rollups: PerfRollups{Stats: []PerfRollupValue{}}},
	)
	s.Require().NoError(err)

	s.NoError(s.r.Add(s.ctx, PerfRollupValue{
		Name:          "float",
		Value:         12.4,
		Version:       1,
		MetricType:    MetricTypeMax,
		UserSubmitted: true,
		Valid:         true,
	}))
	s.NoError(s.r.Add(s.ctx, PerfRollupValue{
		Name:          "int",
		Version:       2,
		Value:         12,
		MetricType:    MetricTypeMax,
		UserSubmitted: true,
		Valid:         true,
	}))
	s.NoError(s.r.Add(s.ctx, PerfRollupValue{
		Name:          "int32",
		Version:       3,
		Value:         int32(32),
		MetricType:    MetricTypeMax,
		UserSubmitted: true,
		Valid:         true,
	}))
	s.NoError(s.r.Add(s.ctx, PerfRollupValue{
		Name:          "long",
		Version:       4,
		Value:         int64(20216),
		MetricType:    MetricTypeMax,
		UserSubmitted: false,
		Valid:         true,
	}))
}

func (s *perfRollupSuite) TearDownTest() {
	defer s.cancel()
	s.NoError(s.r.env.GetDB().Collection(perfResultCollection).Drop(s.ctx))
}

func (s *perfRollupSuite) TestSetupTestIsValid() {
	search := bson.M{
		"_id": s.r.id,
	}
	out := PerformanceResult{}
	err := s.r.env.GetDB().Collection(perfResultCollection).FindOne(s.ctx, search).Decode(&out)
	s.Require().NoError(err)
	s.Require().NotNil(out.Rollups)

	s.Len(out.Rollups.Stats, 4)
	hasFloat, hasInt, hasInt32, hasLong := false, false, false, false
	for _, entry := range out.Rollups.Stats {
		switch entry.Name {
		case "float":
			s.False(hasFloat)
			s.Equal(entry.Version, 1)
			hasFloat = true
		case "int":
			s.False(hasInt)
			s.Equal(entry.Version, 2)
			hasInt = true
		case "int32":
			s.False(hasInt32)
			s.Equal(entry.Version, 3)
			hasInt32 = true
		case "long":
			s.False(hasLong)
			s.Equal(entry.Version, 4)
			hasLong = true
		}
	}
	s.True(hasFloat && hasInt && hasInt32 && hasLong)
}

func (s *perfRollupSuite) TestInts() {
	val, err := s.r.GetInt("int")
	s.NoError(err)
	s.Equal(12, val)
	val, err = s.r.GetInt("int32")
	s.NoError(err)
	s.Equal(32, val)

	val2, err := s.r.GetInt32("int32")
	s.NoError(err)
	s.Equal(int32(32), val2)
	val2, err = s.r.GetInt32("int")
	s.NoError(err)
	s.Equal(int32(12), val2)

	_, err = s.r.GetInt("float")
	s.Error(err)
	_, err = s.r.GetInt32("long")
	s.Error(err)
	_, err = s.r.GetInt("fake")
	s.Error(err)
}

func (s *perfRollupSuite) TestFloat() {
	val, err := s.r.GetFloat("float")
	s.NoError(err)
	s.Equal(12.4, val)

	_, err = s.r.GetFloat("long")
	s.Error(err)
	_, err = s.r.GetFloat("fake")
	s.Error(err)
}

func (s *perfRollupSuite) TestLong() {
	val, err := s.r.GetInt64("long")
	s.NoError(err)
	s.Equal(int64(20216), val)

	_, err = s.r.GetInt64("float")
	s.Error(err)
	_, err = s.r.GetInt64("fake")
	s.Error(err)
}

func (s *perfRollupSuite) TestAddPerfRollupValue() {
	s.Len(s.r.Stats, 4)
	_, err := s.r.GetFloat("mean")
	s.Error(err)
	s.NoError(s.r.Add(s.ctx, PerfRollupValue{
		Name:          "mean",
		Version:       1,
		Value:         12.24,
		MetricType:    MetricTypeMean,
		UserSubmitted: false,
		Valid:         true,
	}))
	val, err := s.r.GetFloat("mean")
	s.NoError(err)
	s.Equal(12.24, val)
	s.Len(s.r.Stats, 5)
}

func (s *perfRollupSuite) TestAddInvalidPerfRollupValue() {
	s.r.id = "invalid"
	c := s.r.env.GetDB().Collection(perfResultCollection)
	_, err := c.InsertOne(s.ctx, bson.M{"_id": s.r.id})
	s.Require().NoError(err)
	s.NoError(s.r.Add(s.ctx, PerfRollupValue{
		Name:          "invalid",
		Version:       1,
		Value:         nil,
		MetricType:    MetricTypeMax,
		UserSubmitted: true,
		Valid:         false,
	}))
	search := bson.M{
		"_id":                s.r.id,
		"rollups.stats.name": "invalid",
	}

	out := PerformanceResult{}
	err = c.FindOne(s.ctx, search).Decode(&out)
	s.Require().NoError(err)
	s.Require().Len(out.Rollups.Stats, 1)
	s.Equal(out.Rollups.Stats[0].Name, "invalid")
	s.Equal(out.Rollups.Stats[0].Version, 1)
	s.Equal(out.Rollups.Stats[0].Value, nil)
	s.Equal(out.Rollups.Stats[0].UserSubmitted, true)
	s.Equal(out.Rollups.Stats[0].Valid, false)
}

func (s *perfRollupSuite) TestMaps() {
	s.NoError(s.r.Add(s.ctx, PerfRollupValue{
		Name:          "mean",
		Version:       1,
		Value:         12.24,
		MetricType:    MetricTypeMean,
		UserSubmitted: true,
		Valid:         true,
	}))

	allFloats := s.r.MapFloat()
	s.Len(allFloats, 5)
	s.NotZero(allFloats["mean"])
	s.NotZero(allFloats["float"])
	s.NotZero(allFloats["int32"])
	s.Zero(allFloats["avg"])
	s.Equal(allFloats["mean"], 12.24)
	s.Equal(allFloats["int"], 12.0)

	allInts := s.r.Map()
	s.Len(allInts, 3)
	s.NotZero(allInts["int"])
	s.NotZero(allInts["int32"])
	s.NotZero(allInts["long"])
	s.Equal(allInts["int"], int64(12))
}

func (s *perfRollupSuite) TestAddWithNilEnv() {
	env := s.r.env
	s.r.env = nil
	defer func() {
		s.r.env = env
	}()
	s.Error(s.r.Add(s.ctx, PerfRollupValue{
		Name:          "mean",
		Version:       4,
		Value:         12.24,
		MetricType:    MetricTypeMax,
		UserSubmitted: true,
		Valid:         true,
	}))
}

func (s *perfRollupSuite) TestUpdateExistingEntry() {
	s.r.id = "234"
	c := s.r.env.GetDB().Collection(perfResultCollection)
	_, err := c.InsertOne(s.ctx, bson.M{"_id": s.r.id})
	s.Require().NoError(err)
	s.NoError(s.r.Add(s.ctx, PerfRollupValue{
		Name:          "mean",
		Version:       4,
		Value:         12.24,
		MetricType:    MetricTypeMax,
		UserSubmitted: true,
		Valid:         true,
	}))

	search := bson.M{
		"_id":                s.r.id,
		"rollups.stats.name": "mean",
	}
	out := PerformanceResult{}
	err = c.FindOne(s.ctx, search).Decode(&out)
	s.Require().NoError(err)
	s.Require().Len(out.Rollups.Stats, 1)
	s.Equal(out.Rollups.Stats[0].Version, 4)
	s.Equal(out.Rollups.Stats[0].Value, 12.24)
	s.Equal(out.Rollups.Stats[0].UserSubmitted, true)

	s.Require().NoError(s.r.Add(s.ctx, PerfRollupValue{
		Name:          "mean",
		Version:       3,
		Value:         24.12,
		MetricType:    MetricTypeMax,
		UserSubmitted: true,
		Valid:         true,
	}))
	s.Require().NoError(c.FindOne(s.ctx, search).Decode(&out))
	s.Require().Len(out.Rollups.Stats, 1)
	s.Equal(out.Rollups.Stats[0].Version, 4)
	s.Equal(out.Rollups.Stats[0].Value, 12.24)
	s.Equal(out.Rollups.Stats[0].UserSubmitted, true)

	s.NoError(s.r.Add(s.ctx, PerfRollupValue{
		Name:          "mean",
		Version:       5,
		Value:         24.12,
		MetricType:    MetricTypeMax,
		UserSubmitted: false,
		Valid:         true,
	}))
	val, err := s.r.GetFloat("mean")
	s.NoError(err)
	s.Equal(24.12, val)
	s.Require().NoError(c.FindOne(s.ctx, search).Decode(&out))
	s.Require().Len(out.Rollups.Stats, 1)
	s.Equal(5, out.Rollups.Stats[0].Version)
	s.Equal(24.12, out.Rollups.Stats[0].Value)
	s.Equal(false, out.Rollups.Stats[0].UserSubmitted)
}

func (s *perfRollupSuite) TestMergeRollups() {
	// without errors
	rollups := []PerfRollupValue{
		{
			Name:       "ops_per_sec",
			Value:      50001.24,
			Version:    1,
			MetricType: MetricTypeThroughput,
		},
		{
			Name:       "latency",
			Value:      5000,
			MetricType: MetricTypeLatency,
		},
	}

	for i := 0; i < 3; i++ {
		result := &PerformanceResult{}
		s.Require().NoError(s.r.env.GetDB().Collection(perfResultCollection).FindOne(s.ctx, bson.M{"_id": s.r.id}).Decode(result))
		result.Setup(s.r.env)
		s.NoError(result.MergeRollups(s.ctx, rollups))
		s.Require().NoError(s.r.env.GetDB().Collection(perfResultCollection).FindOne(s.ctx, bson.M{"_id": s.r.id}).Decode(result))
		count := 0
		s.Require().NotNil(result.Rollups)
		for _, rollup := range result.Rollups.Stats {
			if rollup.Name == "ops_per_sec" || rollup.Name == "latency" {
				count++
			}
		}
		s.Equal(2, count, "iter=%d", i)
		s.True(time.Since(result.Rollups.ProcessedAt) <= time.Minute)
		s.True(result.Rollups.Valid)
	}

	// nil env
	result := &PerformanceResult{}
	s.Error(result.MergeRollups(s.ctx, rollups))
}
