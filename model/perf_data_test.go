package model

import (
	"testing"
	"time"

	"github.com/evergreen-ci/sink"
	"github.com/stretchr/testify/suite"
	"gopkg.in/mgo.v2/bson"
)

type perfRollupSuite struct {
	r *PerfRollups
	suite.Suite
}

func TestPerfRollupSuite(t *testing.T) {
	suite.Run(t, new(perfRollupSuite))
}

func (s *perfRollupSuite) SetupTest() {
	s.r = new(PerfRollups)
	s.r.Setup(sink.GetEnvironment())
	s.r.populated = true
	s.r.id = "123"
	conf, session, err := sink.GetSessionWithConfig(s.r.env)
	s.Require().NoError(err)
	defer session.Close()

	err = session.DB(conf.DatabaseName).C(perfResultCollection).Insert(bson.M{"_id": s.r.id})
	s.Require().NoError(err)

	s.NoError(s.r.Add("float", 1, 12.4))
	s.NoError(s.r.Add("int", 2, 12))
	s.NoError(s.r.Add("int32", 3, int32(32)))
	s.NoError(s.r.Add("long", 4, int64(20216)))
}

func (s *perfRollupSuite) TestSetupTestIsValid() {
	conf, session, err := sink.GetSessionWithConfig(s.r.env)
	s.Require().NoError(err)
	defer session.Close()
	c := session.DB(conf.DatabaseName).C(perfResultCollection)

	search := bson.M{
		"_id":          s.r.id,
		"rollups.name": "int",
	}
	filter := bson.M{"rollups.version": 1, "rollups.name": 1, "_id": 0}

	out := struct {
		Rollups perfRollupEntries `bson:"rollups"`
	}{}
	err = c.Find(search).Select(filter).One(&out)
	s.Require().NoError(err)
	s.Len(out.Rollups, 4)
	hasFloat, hasInt, hasInt32, hasLong := false, false, false, false
	for _, entry := range out.Rollups {
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
	err = s.r.Add("mean", 1, 12.24)
	s.NoError(err)
	val, err := s.r.GetFloat("mean")
	s.NoError(err)
	s.Equal(12.24, val)
	s.Len(s.r.Stats, 5)
}

func (s *perfRollupSuite) TestMaps() {
	err := s.r.Add("mean", 1, 12.24)
	s.NoError(err)

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

func (s *perfRollupSuite) TestUpdateExistingEntry() {
	s.r.id = "234"
	conf, session, err := sink.GetSessionWithConfig(s.r.env)
	s.Require().NoError(err)
	defer session.Close()

	c := session.DB(conf.DatabaseName).C(perfResultCollection)
	err = c.Insert(bson.M{"_id": s.r.id})
	s.Require().NoError(err)
	err = s.r.Add("mean", 4, 12.24)
	s.NoError(err)

	search := bson.M{
		"_id":          s.r.id,
		"rollups.name": "mean",
	}
	filter := bson.M{"rollups": 1, "_id": 0}
	out := struct {
		Rollups perfRollupEntries `bson:"rollups"`
	}{}
	err = c.Find(search).Select(filter).One(&out)
	s.Require().NoError(err)
	s.Require().Len(out.Rollups, 1)
	s.Equal(out.Rollups[0].Version, 4)
	s.Equal(out.Rollups[0].Value, 12.24)
	err = s.r.Add("mean", 3, 24.12) // should fail with older version
	s.Error(err)

	err = s.r.Add("mean", 5, 24.12)
	s.NoError(err)
	val, err := s.r.GetFloat("mean")
	s.NoError(err)
	s.Equal(24.12, val)
	err = c.Find(search).Select(filter).One(&out)
	s.Require().NoError(err)
	s.Require().Len(out.Rollups, 1)
	s.Equal(5, out.Rollups[0].Version)
	s.Equal(24.12, out.Rollups[0].Value)
}

func (s *perfRollupSuite) TearDownTest() {
	conf, session, err := sink.GetSessionWithConfig(s.r.env)
	s.Require().NoError(err)
	defer session.Close()

	c := session.DB(conf.DatabaseName).C(perfResultCollection)
	err = c.DropCollection()
	s.NoError(err)
}

func (s *perfRollupSuite) TestValidate() {
	err := s.r.Validate()
	s.NoError(err)
	s.Equal(s.r.Count, 4)

	s.r.Count++
	err = s.r.Validate()
	s.Error(err)
}

func initializeTS() PerformanceTimeSeries {
	point1 := PerformancePoint{Timestamp: time.Date(2018, 10, 15, 8, 0, 0, 0, time.Local)}
	point2 := PerformancePoint{Timestamp: point1.Timestamp.Add(time.Minute)}
	point3 := PerformancePoint{Timestamp: point2.Timestamp.Add(time.Minute)}
	point1.Timers.Duration = time.Hour
	point2.Timers.Duration = time.Hour
	point3.Timers.Duration = time.Hour
	point1.Timers.Total = time.Hour
	point2.Timers.Total = time.Hour
	point3.Timers.Total = time.Hour
	point1.Counters.Operations = 200
	point2.Counters.Operations = 400
	point3.Counters.Operations = 600
	point1.Counters.Size = 1000
	point2.Counters.Size = 2000
	point3.Counters.Size = 3000
	point1.Counters.Errors = 400
	point2.Counters.Errors = 300
	point3.Counters.Errors = 200

	return []PerformancePoint{point1, point2, point3}
}

func (s *perfRollupSuite) TestUpdateDefaultRollups() {
	r := new(PerfRollups)
	r.Setup(sink.GetEnvironment())
	r.populated = true
	r.id = "345"
	conf, session, err := sink.GetSessionWithConfig(r.env)
	s.Require().NoError(err)
	defer session.Close()

	err = session.DB(conf.DatabaseName).C(perfResultCollection).Insert(bson.M{"_id": r.id})
	s.Require().NoError(err)

	ts := initializeTS()
	result := PerformanceResult{
		Rollups: r,
	}
	s.NoError(result.UpdateDefaultRollups(ts))

	rollups := r.MapFloat()
	span := (2 * time.Minute).Seconds()
	s.Require().Len(rollups, 12)
	s.Equal((3 * time.Hour).Seconds(), rollups["totalTime"])
	s.Equal(3.0, rollups["totalSamples"])
	s.Equal(300.0/span, rollups["errorRate_mean"])

	// test update of previous rollup
	ts[0].Counters.Size = 10000
	s.NoError(result.UpdateDefaultRollups(ts))
	rollups2 := r.MapFloat()
	s.Require().Len(rollups2, 12)
	s.Equal(rollups["errorRate_mean"], rollups2["errorRate_mean"])
	s.NotEqual(rollups["throughputSize_mean"], rollups2["throughputSize_mean"])

	ts[0].Timestamp = ts[2].Timestamp
	s.Error(result.UpdateDefaultRollups(ts))
}
