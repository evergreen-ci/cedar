package model

import (
	"testing"
	"time"

	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/util"
	"github.com/stretchr/testify/suite"
)

type perfResultSuite struct {
	r *PerformanceResults
	suite.Suite
}

func TestPerfResultSuite(t *testing.T) {
	suite.Run(t, new(perfResultSuite))
}

func getTimeForTestingByDate(day int) time.Time {
	return time.Date(2018, 10, day, 0, 0, 0, 0, time.Local)
}

func (s *perfResultSuite) SetupTest() {
	s.r = new(PerformanceResults)
	s.r.Setup(sink.GetEnvironment())
	args := make(map[string]int32)
	args["timeout"] = 12
	info := PerformanceResultInfo{
		Parent:    "123",
		Version:   "1",
		Project:   "test",
		Trial:     12,
		Tags:      []string{"tag1", "bson"},
		Arguments: args,
	}
	source := []ArtifactInfo{}
	result := CreatePerformanceResult(info, source)
	result.Setup(sink.GetEnvironment())
	result.CreatedAt = getTimeForTestingByDate(15)
	result.Version = 1
	s.True(result.CompletedAt.IsZero())
	s.NoError(result.Save())

	info = PerformanceResultInfo{
		Parent:  "234",
		Version: "1",
		Trial:   10,
		Tags:    []string{"tag2", "json"},
	}
	result2 := CreatePerformanceResult(info, source)
	result2.Setup(sink.GetEnvironment())
	result2.CreatedAt = getTimeForTestingByDate(16)
	result2.CompletedAt = getTimeForTestingByDate(18)
	result2.Version = 2
	s.NoError(result2.Save())
}

func (s *perfResultSuite) TestSavePerfResults() {
	info := PerformanceResultInfo{Parent: "345"}
	source := []ArtifactInfo{}
	result := CreatePerformanceResult(info, source)
	result.Setup(sink.GetEnvironment())
	result.CreatedAt = getTimeForTestingByDate(12)
	s.NoError(result.Save())
	s.NoError(result.Find())
}

func (s *perfResultSuite) TestFindResultsByTimeInterval() {
	start := getTimeForTestingByDate(15)
	options := PerfFindOptions{
		Interval: util.GetTimeRange(start, time.Hour*48),
	}
	s.NoError(s.r.Find(options))
	s.Require().Len(s.r.Results, 1)
	s.Equal(s.r.Results[0].Version, 1)

	start = getTimeForTestingByDate(16)
	options.Interval = util.GetTimeRange(start, time.Hour*48)
	s.NoError(s.r.Find(options))
	s.Require().Len(s.r.Results, 1)
	s.Equal(s.r.Results[0].Version, 2)

	start = getTimeForTestingByDate(15)
	options.Interval = util.GetTimeRange(start, time.Hour*72)
	s.NoError(s.r.Find(options))
	s.Len(s.r.Results, 2)

	options.Interval = util.GetTimeRange(start, -time.Hour*24)
	s.Error(s.r.Find(options))
}

func (s *perfResultSuite) TestFindResultsWithOptionsInfo() {
	start := getTimeForTestingByDate(15)
	options := PerfFindOptions{
		Interval: util.GetTimeRange(start, time.Hour*72),
	}
	options.Info.Version = "1"
	s.NoError(s.r.Find(options))
	s.Len(s.r.Results, 2)

	options.Info.Tags = []string{"tag1", "tag2", "tag3"}
	s.NoError(s.r.Find(options))
	s.Len(s.r.Results, 2)

	options.Info.Project = "test"
	s.NoError(s.r.Find(options))
	s.Require().Len(s.r.Results, 1)
	s.Equal(s.r.Results[0].Info.Version, "1")

	options.Info = PerformanceResultInfo{}
	options.Info.Trial = 10
	s.Len(s.r.Results, 1)
	options.Info = PerformanceResultInfo{}
	options.Info.Arguments = make(map[string]int32)
	options.Info.Arguments["timeout"] = 12
	options.Info.Arguments["something"] = 24
	s.NoError(s.r.Find(options))
	s.Require().Len(s.r.Results, 1)
	s.Equal(s.r.Results[0].Info.Version, "1")
}

func (s *perfResultSuite) TestSearchResultsWithParent() {
	// nodeA -> nodeB and nodeC, nodeB -> nodeD
	s.r = new(PerformanceResults)
	s.r.Setup(sink.GetEnvironment())

	info := PerformanceResultInfo{Parent: "NA"}
	source := []ArtifactInfo{}
	nodeA := CreatePerformanceResult(info, source)
	nodeA.Setup(sink.GetEnvironment())
	nodeA.CreatedAt = getTimeForTestingByDate(15)
	s.NoError(nodeA.Save())

	info = PerformanceResultInfo{Parent: nodeA.ID}
	nodeB := CreatePerformanceResult(info, []ArtifactInfo{})
	nodeB.Setup(sink.GetEnvironment())
	nodeB.CreatedAt = getTimeForTestingByDate(16)
	s.NoError(nodeB.Save())

	info.Version = "C"
	nodeC := CreatePerformanceResult(info, []ArtifactInfo{})
	nodeC.Setup(sink.GetEnvironment())
	nodeC.CreatedAt = getTimeForTestingByDate(16)
	s.NoError(nodeC.Save())

	info = PerformanceResultInfo{Parent: nodeB.ID}
	nodeD := CreatePerformanceResult(info, []ArtifactInfo{})
	nodeD.Setup(sink.GetEnvironment())
	nodeD.CreatedAt = getTimeForTestingByDate(17)
	s.NoError(nodeD.Save())

	start := getTimeForTestingByDate(15)
	options := PerfFindOptions{
		Interval: util.GetTimeRange(start, time.Hour*72),
	}
	options.Info.Parent = nodeA.ID
	s.NoError(s.r.Find(options))
	s.Require().Len(s.r.Results, 4)
	s.Equal(s.r.Results[0].ID, nodeA.ID)
	s.Equal(s.r.Results[1].Info.Parent, nodeA.ID)
	s.Equal(s.r.Results[2].Info.Parent, nodeA.ID)
	s.Equal(s.r.Results[3].ID, nodeD.ID)
}

func (s *perfResultSuite) TearDownTest() {
	conf, session, err := sink.GetSessionWithConfig(s.r.env)
	s.Require().NoError(err)
	defer session.Close()
	c := session.DB(conf.DatabaseName).C(perfResultCollection)
	err = c.DropCollection()
	s.NoError(err)
}
