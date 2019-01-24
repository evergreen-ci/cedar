package model

import (
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/util"
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
	s.r.Setup(cedar.GetEnvironment())
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
	result.Setup(cedar.GetEnvironment())
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
	result2.Setup(cedar.GetEnvironment())
	result2.CreatedAt = getTimeForTestingByDate(16)
	result2.CompletedAt = getTimeForTestingByDate(18)
	result2.Version = 2
	s.NoError(result2.Save())
}

func (s *perfResultSuite) TestSavePerfResult() {
	info := PerformanceResultInfo{Parent: "345"}
	source := []ArtifactInfo{}
	result := CreatePerformanceResult(info, source)
	result.Setup(cedar.GetEnvironment())
	result.CreatedAt = getTimeForTestingByDate(12)
	s.NoError(result.Save())
	s.NoError(result.Find())
}

func (s *perfResultSuite) TestRemovePerfResult() {
	// save result
	info := PerformanceResultInfo{Parent: "345"}
	source := []ArtifactInfo{}
	result := CreatePerformanceResult(info, source)
	result.Setup(cedar.GetEnvironment())
	result.CreatedAt = getTimeForTestingByDate(12)
	s.NoError(result.Save())
	s.NoError(result.Find())

	// remove
	result.Setup(cedar.GetEnvironment())
	s.NoError(result.Remove())

	// check if exists
	s.Error(result.Find())

	// remove again should not return error
	result = CreatePerformanceResult(info, source)
	result.Setup(cedar.GetEnvironment())
	result.CreatedAt = getTimeForTestingByDate(12)
	s.NoError(result.Remove())
}

func (s *perfResultSuite) TestFindResultsByTimeInterval() {
	start := getTimeForTestingByDate(15)
	options := PerfFindOptions{
		Interval: util.GetTimeRange(start, time.Hour*48),
		MaxDepth: 5,
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
		MaxDepth: 5,
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
	s.r.Setup(cedar.GetEnvironment())

	info := PerformanceResultInfo{Parent: "NA"}
	source := []ArtifactInfo{}
	nodeA := CreatePerformanceResult(info, source)
	nodeA.Setup(cedar.GetEnvironment())
	nodeA.CreatedAt = getTimeForTestingByDate(15)
	s.NoError(nodeA.Save())

	info = PerformanceResultInfo{
		Parent: nodeA.ID,
		Tags:   []string{"tag0"},
	}
	nodeB := CreatePerformanceResult(info, []ArtifactInfo{})
	nodeB.Setup(cedar.GetEnvironment())
	nodeB.CreatedAt = getTimeForTestingByDate(16)
	s.NoError(nodeB.Save())

	info.Version = "C"
	nodeC := CreatePerformanceResult(info, []ArtifactInfo{})
	nodeC.Setup(cedar.GetEnvironment())
	nodeC.CreatedAt = getTimeForTestingByDate(16)
	s.NoError(nodeC.Save())

	info = PerformanceResultInfo{
		Parent: nodeB.ID,
		Tags:   []string{"tag1"},
	}
	nodeD := CreatePerformanceResult(info, []ArtifactInfo{})
	nodeD.Setup(cedar.GetEnvironment())
	nodeD.CreatedAt = getTimeForTestingByDate(17)
	s.NoError(nodeD.Save())

	// Without $graphLookup
	options := PerfFindOptions{
		MaxDepth: 5,
	}
	options.Info.Parent = nodeA.ID
	s.NoError(s.r.Find(options))
	s.Require().Len(s.r.Results, 4)
	s.Equal(s.r.Results[0].ID, nodeA.ID)
	s.Equal(s.r.Results[1].Info.Parent, nodeA.ID)
	s.Equal(s.r.Results[2].Info.Parent, nodeA.ID)
	s.Equal(s.r.Results[3].ID, nodeD.ID)

	// Test min through max depth without $graphLookup
	options = PerfFindOptions{
		MaxDepth: 0,
	}
	options.Info.Parent = nodeA.ID
	s.NoError(s.r.Find(options))
	s.Require().Len(s.r.Results, 1)
	s.Equal(s.r.Results[0].ID, nodeA.ID)

	options = PerfFindOptions{
		MaxDepth: 1,
	}
	options.Info.Parent = nodeA.ID
	s.NoError(s.r.Find(options))
	s.Require().Len(s.r.Results, 3)
	s.Equal(s.r.Results[0].ID, nodeA.ID)
	s.Equal(s.r.Results[1].Info.Parent, nodeA.ID)
	s.Equal(s.r.Results[2].Info.Parent, nodeA.ID)

	options = PerfFindOptions{
		MaxDepth: -1,
	}
	options.Info.Parent = nodeA.ID
	s.NoError(s.r.Find(options))
	s.Require().Len(s.r.Results, 4)
	s.Equal(s.r.Results[0].ID, nodeA.ID)
	s.Equal(s.r.Results[1].Info.Parent, nodeA.ID)
	s.Equal(s.r.Results[2].Info.Parent, nodeA.ID)
	s.Equal(s.r.Results[3].ID, nodeD.ID)

	// With $graphLookup
	options = PerfFindOptions{
		MaxDepth:    5,
		GraphLookup: true,
	}
	options.Info.Parent = nodeA.ID
	s.NoError(s.r.Find(options))
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
	s.NoError(s.r.Find(options))
	s.Require().Len(s.r.Results, 1)
	s.Equal(s.r.Results[0].ID, nodeA.ID)

	options = PerfFindOptions{
		MaxDepth:    1,
		GraphLookup: true,
	}
	options.Info.Parent = nodeA.ID
	s.NoError(s.r.Find(options))
	s.Require().Len(s.r.Results, 3)
	s.Equal(s.r.Results[0].ID, nodeA.ID)
	s.Equal(s.r.Results[1].Info.Parent, nodeA.ID)
	s.Equal(s.r.Results[2].Info.Parent, nodeA.ID)

	options = PerfFindOptions{
		MaxDepth:    -1,
		GraphLookup: true,
	}
	options.Info.Parent = nodeA.ID
	s.NoError(s.r.Find(options))
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
	s.NoError(s.r.Find(options))
	s.Require().Len(s.r.Results, 2)
	s.Equal(s.r.Results[0].ID, nodeA.ID)
	s.Equal(s.r.Results[1].ID, nodeD.ID)
}

func (s *perfResultSuite) TearDownTest() {
	conf, session, err := cedar.GetSessionWithConfig(s.r.env)
	s.Require().NoError(err)
	defer session.Close()
	c := session.DB(conf.DatabaseName).C(perfResultCollection)
	err = c.DropCollection()
	s.NoError(err)
}
