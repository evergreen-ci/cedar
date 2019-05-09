package model

import (
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/util"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/grip"
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
	env := cedar.GetEnvironment()

	// ctx, cancel := env.Context()
	// defer cancel()
	// db, err := env.GetDB()
	// s.Require().NoError(err)
	// s.Require().NoError(db.Collection(perfResultCollection).Drop(ctx))

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
	s.NoError(result.Save())

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
	s.NoError(result2.Save())

	info = PerformanceResultInfo{
		Version:  "1",
		Order:    3,
		TaskName: "task",
		Mainline: true,
	}
	result3 := CreatePerformanceResult(info, source, nil)
	result3.Setup(cedar.GetEnvironment())
	s.NoError(result3.Save())

}

func (s *perfResultSuite) TearDownTest() {
	conf, session, err := cedar.GetSessionWithConfig(s.r.env)
	s.Require().NoError(err)
	defer session.Close()
	c := session.DB(conf.DatabaseName).C(perfResultCollection)
	err = c.DropCollection()
	s.Require().NoError(err)
}

func (s *perfResultSuite) TestSavePerfResult() {
	info := PerformanceResultInfo{Parent: "345"}
	source := []ArtifactInfo{}
	result := CreatePerformanceResult(info, source, nil)
	result.Setup(cedar.GetEnvironment())
	result.CreatedAt = getTimeForTestingByDate(12)
	s.NoError(result.Save())
	s.NoError(result.Find())
}

func (s *perfResultSuite) TestRemovePerfResult() {
	// save result
	info := PerformanceResultInfo{Parent: "345"}
	source := []ArtifactInfo{}
	result := CreatePerformanceResult(info, source, nil)
	result.Setup(cedar.GetEnvironment())
	result.CreatedAt = getTimeForTestingByDate(12)
	s.NoError(result.Save())
	s.NoError(result.Find())

	// remove
	result.Setup(cedar.GetEnvironment())
	numRemoved, err := result.Remove()
	s.NoError(err)
	s.Equal(1, numRemoved)

	// check if exists
	s.Error(result.Find())

	// remove again should not return error
	result = CreatePerformanceResult(info, source, nil)
	result.Setup(cedar.GetEnvironment())
	result.CreatedAt = getTimeForTestingByDate(12)
	numRemoved, err = result.Remove()
	s.NoError(err)
	s.Equal(0, numRemoved)
}

func (s *perfResultSuite) TestFindResultsByTimeInterval() {
	start := getTimeForTestingByDate(15)
	options := PerfFindOptions{
		Interval: util.GetTimeRange(start, time.Hour*48),
		MaxDepth: 5,
	}
	s.NoError(s.r.Find(options))
	s.Require().Len(s.r.Results, 1)
	s.Equal(1, s.r.Results[0].Version)

	start = getTimeForTestingByDate(16)
	options.Interval = util.GetTimeRange(start, time.Hour*48)
	s.NoError(s.r.Find(options))
	s.Require().Len(s.r.Results, 1)
	s.Equal(2, s.r.Results[0].Version)

	start = getTimeForTestingByDate(15)
	options.Interval = util.GetTimeRange(start, time.Hour*72)
	s.NoError(s.r.Find(options))
	s.Len(s.r.Results, 2)
	options.Limit = 1
	s.NoError(s.r.Find(options))
	s.Len(s.r.Results, 1)

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
	s.Equal("1", s.r.Results[0].Info.Version, "%+v", options)

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

func (s *perfResultSuite) TestFindResultsWithSortAndLimit() {
	options := PerfFindOptions{
		Interval: util.TimeRange{
			StartAt: time.Time{},
			EndAt:   time.Now(),
		},
		Info: PerformanceResultInfo{TaskName: "task"},
		Sort: []string{"-" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoOrderKey)},
	}
	s.NoError(s.r.Find(options))
	s.Len(s.r.Results, 3)
	for i := 1; i < len(s.r.Results); i++ {
		s.True(s.r.Results[i-1].Info.Order >= s.r.Results[i].Info.Order)
	}

	options.Limit = 2
	s.NoError(s.r.Find(options))
	s.Len(s.r.Results, 2)
	for i := 1; i < len(s.r.Results); i++ {
		s.True(s.r.Results[i-1].Info.Order >= s.r.Results[i].Info.Order)
	}

	options.Sort = []string{bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoOrderKey)}
	options.Limit = 0
	s.NoError(s.r.Find(options))
	s.Len(s.r.Results, 3)
	for i := 1; i < len(s.r.Results); i++ {
		s.True(s.r.Results[i-1].Info.Order <= s.r.Results[i].Info.Order)
	}
}

func (s *perfResultSuite) TestSearchResultsWithParent() {
	// nodeA -> nodeB and nodeC, nodeB -> nodeD
	s.r = new(PerformanceResults)
	s.r.Setup(cedar.GetEnvironment())

	info := PerformanceResultInfo{Parent: "NA"}
	source := []ArtifactInfo{}
	nodeA := CreatePerformanceResult(info, source, nil)
	nodeA.Setup(cedar.GetEnvironment())
	nodeA.CreatedAt = getTimeForTestingByDate(15)
	s.NoError(nodeA.Save())

	info = PerformanceResultInfo{
		Parent: nodeA.ID,
		Tags:   []string{"tag0"},
	}
	nodeB := CreatePerformanceResult(info, []ArtifactInfo{}, nil)
	nodeB.Setup(cedar.GetEnvironment())
	nodeB.CreatedAt = getTimeForTestingByDate(16)
	s.NoError(nodeB.Save())

	info.Version = "C"
	nodeC := CreatePerformanceResult(info, []ArtifactInfo{}, nil)
	nodeC.Setup(cedar.GetEnvironment())
	nodeC.CreatedAt = getTimeForTestingByDate(16)
	s.NoError(nodeC.Save())

	info = PerformanceResultInfo{
		Parent: nodeB.ID,
		Tags:   []string{"tag1"},
	}
	nodeD := CreatePerformanceResult(info, []ArtifactInfo{}, nil)
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
	s.NoError(s.r.Find(options))
	s.Require().Len(s.r.Results, 1)
	s.Equal(s.r.Results[0].ID, nodeA.ID)

	options = PerfFindOptions{
		MaxDepth: 1,
	}
	options.Info.Parent = nodeA.ID
	s.NoError(s.r.Find(options))
	s.Require().Len(s.r.Results, 3)
	grip.Notice(s.r.Results)
	s.Equal(s.r.Results[0].ID, nodeA.ID)
	s.Equal(s.r.Results[1].Info.Parent, nodeA.ID)
	s.Equal(s.r.Results[2].Info.Parent, nodeA.ID)

	options = PerfFindOptions{
		MaxDepth: -1,
	}
	options.Info.Parent = nodeA.ID
	s.NoError(s.r.Find(options))
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

	// Test remove removes all children
	root := PerformanceResult{ID: nodeA.ID}
	root.Setup(cedar.GetEnvironment())
	numRemoved, err := root.Remove()
	s.NoError(err)
	s.Equal(4, numRemoved)
	options = PerfFindOptions{
		MaxDepth:    -1,
		GraphLookup: true,
	}
	options.Info.Parent = nodeA.ID
	s.NoError(s.r.Find(options))
	s.Len(s.r.Results, 0)
}

func (s *perfResultSuite) TestFindOutdated() {
	s.TearDownTest()
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
	s.Require().NoError(result.Save())

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
	s.Require().NoError(result.Save())

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
	s.Require().NoError(result.Save())

	outdated := PerformanceResultInfo{Project: "Outdated"}
	result = CreatePerformanceResult(outdated, source, nil)
	result.CreatedAt = time.Now()
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
	s.Require().NoError(result.Save())

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
	s.Require().NoError(result.Save())

	s.Require().NoError(s.r.FindOutdatedRollups(rollupName, 2, time.Now().Add(-time.Hour)))
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
	s.Require().NoError(result.Save())

	s.Require().NoError(s.r.FindOutdatedRollups("DNE", 1, time.Now().Add(-2*time.Hour)))
	s.Require().Len(s.r.Results, 4)
	for _, result := range s.r.Results {
		s.NotEqual(doesNotExist.ID(), result.Info.ID())
	}
}
