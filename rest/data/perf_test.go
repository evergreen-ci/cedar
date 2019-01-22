package data

import (
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	dataModel "github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/cedar/util"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/suite"
)

func createEnv() (cedar.Environment, error) {
	env := cedar.GetEnvironment()
	err := env.Configure(&cedar.Configuration{
		MongoDBURI:    "mongodb://localhost:27017",
		DatabaseName:  "grpc_test",
		SocketTimeout: time.Hour,
		NumWorkers:    2,
		UseLocalQueue: true,
	})
	return env, errors.WithStack(err)
}

func tearDownEnv(env cedar.Environment) error {
	conf, session, err := cedar.GetSessionWithConfig(env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()
	return errors.WithStack(session.DB(conf.DatabaseName).DropDatabase())
}

type testResults []struct {
	info   *model.PerformanceResultInfo
	parent int
}

func (s *PerfConnectorSuite) createPerformanceResults(env cedar.Environment) error {
	s.idMap = map[string]dataModel.APIPerformanceResult{}
	results := testResults{
		{
			info: &model.PerformanceResultInfo{
				Project:  "test",
				Version:  "0",
				TaskName: "task0",
				TaskID:   "task1",
				Tags:     []string{"tag1", "tag2"},
			},
			parent: -1,
		},
		{
			info: &model.PerformanceResultInfo{
				Project:  "test",
				Version:  "0",
				TaskName: "task1",
				TaskID:   "task1",
				Tags:     []string{"tag2"},
			},
			parent: 0,
		},
		{
			info: &model.PerformanceResultInfo{
				Project:  "test",
				Version:  "1",
				TaskName: "task2",
				TaskID:   "task1",
				Tags:     []string{"tag1"},
			},
			parent: 0,
		},
		{
			info: &model.PerformanceResultInfo{
				Project:  "test",
				Version:  "0",
				TaskName: "task3",
				TaskID:   "task1",
				Tags:     []string{"tag3"},
			},
			parent: 1,
		},
		{
			info: &model.PerformanceResultInfo{
				Project:  "test",
				Version:  "1",
				TaskName: "task0",
				TaskID:   "task2",
				Tags:     []string{"tag3"},
			},
			parent: -1,
		},
	}

	for _, result := range results {
		if result.parent > -1 {
			result.info.Parent = results[result.parent].info.ID()
		}
		performanceResult := model.CreatePerformanceResult(*result.info, []model.ArtifactInfo{})
		performanceResult.CreatedAt = time.Now().Add(time.Second * -1)
		performanceResult.Setup(env)
		err := performanceResult.Save()
		if err != nil {
			return errors.WithStack(err)
		}
		apiResult := dataModel.APIPerformanceResult{}
		err = apiResult.Import(*performanceResult)
		s.Require().NoError(err)
		s.idMap[performanceResult.ID] = apiResult
	}
	s.results = results
	return nil
}

func (s *PerfConnectorSuite) createChildMap() {
	s.childMap = map[string][]string{}
	for _, result := range s.results {
		if result.parent < 0 {
			continue
		}
		parentId := s.results[result.parent].info.ID()
		s.childMap[parentId] = append(s.childMap[parentId], result.info.ID())
	}
}

func (s *PerfConnectorSuite) getLineage(id string) map[string]bool {
	lineage := make(map[string]bool)
	lineage[id] = true
	queue := []string{id}

	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		children, ok := s.childMap[next]
		if ok {
			queue = append(queue, children...)
			for _, child := range children {
				lineage[child] = true
			}
		}
	}
	return lineage
}

type PerfConnectorSuite struct {
	sc       Connector
	env      cedar.Environment
	results  testResults
	idMap    map[string]dataModel.APIPerformanceResult
	childMap map[string][]string

	suite.Suite
}

func (s *PerfConnectorSuite) setup() {
	env, err := createEnv()
	s.Require().NoError(err)
	s.env = env
	err = s.createPerformanceResults(env)
	s.Require().NoError(err)
	s.createChildMap()
}

func (s *PerfConnectorSuite) TearDownSuite() {
	err := tearDownEnv(s.env)
	s.Require().NoError(err)
}

func TestPerfConnectorSuiteDB(t *testing.T) {
	s := new(PerfConnectorSuite)
	s.setup()
	s.sc = CreateNewDBConnector(s.env)
	suite.Run(t, s)
}

func TestPerfConnectorSuiteMock(t *testing.T) {
	s := new(PerfConnectorSuite)
	s.setup()
	s.sc = &MockConnector{
		CachedPerformanceResults: s.idMap,
		ChildMap:                 s.childMap,
	}
	suite.Run(t, s)
}

func (s *PerfConnectorSuite) TestFindPerformanceResultById() {
	expectedID := s.results[0].info.ID()

	actualResult, err := s.sc.FindPerformanceResultById(expectedID)
	s.Require().NoError(err)
	s.Equal(expectedID, *actualResult.Name)

	actualResult, err = s.sc.FindPerformanceResultById(expectedID)
	s.Require().NoError(err)
	s.Equal(expectedID, *actualResult.Name)
}

func (s *PerfConnectorSuite) TestFindPerformanceResultByIdDoesNotExist() {
	var expectedResult *dataModel.APIPerformanceResult
	actualResult, err := s.sc.FindPerformanceResultById("doesNotExist")
	s.Equal(expectedResult, actualResult)
	s.Error(err)
}

func (s *PerfConnectorSuite) TestFindPerformanceResultsByTaskId() {
	expectedTaskID := s.results[0].info.TaskID
	expectedCount := 0
	for _, result := range s.results {
		if result.info.TaskID == expectedTaskID {
			expectedCount++
		}
	}
	dur, err := time.ParseDuration("100h")
	s.Require().NoError(err)
	tr := util.GetTimeRange(time.Time{}, dur)

	actualResult, err := s.sc.FindPerformanceResultsByTaskId(expectedTaskID, tr)
	s.Equal(expectedCount, len(actualResult))
	for _, result := range actualResult {
		s.Equal(expectedTaskID, *result.Info.TaskID)
	}
	s.NoError(err)

	// Now with tags
	actualResult, err = s.sc.FindPerformanceResultsByTaskId(expectedTaskID, tr, "tag1", "tag2")
	s.True(len(actualResult) < expectedCount)
	for _, result := range actualResult {
		s.Equal(expectedTaskID, *result.Info.TaskID)
		foundTag := false
		for _, tag := range result.Info.Tags {
			if tag == "tag1" || tag == "tag2" {
				foundTag = true
				break
			}
		}
		s.True(foundTag)
	}
	s.NoError(err)
}

func (s *PerfConnectorSuite) TestFindPerformanceResultsByTaskIdDoesNotExist() {
	dur, err := time.ParseDuration("100h")
	s.Require().NoError(err)
	tr := util.GetTimeRange(time.Time{}, dur)

	var expectedResult []dataModel.APIPerformanceResult
	actualResult, err := s.sc.FindPerformanceResultsByTaskId("doesNotExist", tr)
	s.Equal(expectedResult, actualResult)
	s.Error(err)
}

func (s *PerfConnectorSuite) TestFindPerformanceResultsByTaskIdNarrowInterval() {
	dur, err := time.ParseDuration("1ns")
	s.Require().NoError(err)
	tr := util.GetTimeRange(time.Time{}, dur)

	var expectedResult []dataModel.APIPerformanceResult
	actualResult, err := s.sc.FindPerformanceResultsByTaskId(s.results[0].info.TaskID, tr)
	s.Equal(expectedResult, actualResult)
	s.Error(err)
}

func (s *PerfConnectorSuite) TestFindPerformanceResultsByTaskName() {
	expectedTaskName := s.results[0].info.TaskName
	expectedCount := 0
	for _, result := range s.results {
		if result.info.TaskName == expectedTaskName {
			expectedCount++
		}
	}
	dur, err := time.ParseDuration("100h")
	s.Require().NoError(err)
	tr := util.GetTimeRange(time.Time{}, dur)

	actualResult, err := s.sc.FindPerformanceResultsByTaskName(expectedTaskName, tr)
	s.Equal(expectedCount, len(actualResult))
	for _, result := range actualResult {
		s.Equal(expectedTaskName, *result.Info.TaskName)
	}
	s.NoError(err)

	// Now with tags
	actualResult, err = s.sc.FindPerformanceResultsByTaskName(expectedTaskName, tr, "tag1", "tag2")
	s.True(len(actualResult) < expectedCount)
	for _, result := range actualResult {
		s.Equal(expectedTaskName, *result.Info.TaskName)
		foundTag := false
		for _, tag := range result.Info.Tags {
			if tag == "tag1" || tag == "tag2" {
				foundTag = true
				break
			}
		}
		s.True(foundTag)
	}
	s.NoError(err)
}

func (s *PerfConnectorSuite) TestFindPerformanceResultsByTaskNameDoesNotExist() {
	dur, err := time.ParseDuration("100h")
	s.Require().NoError(err)
	tr := util.GetTimeRange(time.Time{}, dur)

	var expectedResult []dataModel.APIPerformanceResult
	actualResult, err := s.sc.FindPerformanceResultsByTaskName("doesNotExist", tr)
	s.Equal(expectedResult, actualResult)
	s.Error(err)
}

func (s *PerfConnectorSuite) TestFindPerformanceResultsByTaskNameNarrowInterval() {
	dur, err := time.ParseDuration("1ns")
	s.Require().NoError(err)
	tr := util.GetTimeRange(time.Time{}, dur)

	var expectedResult []dataModel.APIPerformanceResult
	actualResult, err := s.sc.FindPerformanceResultsByTaskName(s.results[0].info.TaskName, tr)
	s.Equal(expectedResult, actualResult)
	s.Error(err)
}
func (s *PerfConnectorSuite) TestFindPerformanceResultsByVersion() {
	expectedVersion := s.results[0].info.Version
	expectedCount := 0
	for _, result := range s.results {
		if result.info.Version == expectedVersion {
			expectedCount++
		}
	}
	dur, err := time.ParseDuration("100h")
	s.Require().NoError(err)
	tr := util.GetTimeRange(time.Time{}, dur)

	actualResult, err := s.sc.FindPerformanceResultsByVersion(expectedVersion, tr)
	s.Equal(expectedCount, len(actualResult))
	for _, result := range actualResult {
		s.Equal(expectedVersion, *result.Info.Version)
	}
	s.NoError(err)

	// Now with tags
	actualResult, err = s.sc.FindPerformanceResultsByVersion(expectedVersion, tr, "tag1")
	s.True(len(actualResult) < expectedCount)
	for _, result := range actualResult {
		s.Equal(expectedVersion, *result.Info.Version)
		foundTag := false
		for _, tag := range result.Info.Tags {
			if tag == "tag1" {
				foundTag = true
				break
			}
		}
		s.True(foundTag)
	}
	s.NoError(err)
}

func (s *PerfConnectorSuite) TestFindPerformanceResultsByVersionDoesNotExist() {
	dur, err := time.ParseDuration("100h")
	s.Require().NoError(err)
	tr := util.GetTimeRange(time.Time{}, dur)

	var expectedResult []dataModel.APIPerformanceResult
	actualResult, err := s.sc.FindPerformanceResultsByVersion("doesNotExist", tr)
	s.Equal(expectedResult, actualResult)
	s.Error(err)
}

func (s *PerfConnectorSuite) TestFindPerformanceResultsByVersionNarrowInterval() {
	dur, err := time.ParseDuration("1ns")
	s.Require().NoError(err)
	tr := util.GetTimeRange(time.Time{}, dur)

	var expectedResult []dataModel.APIPerformanceResult
	actualResult, err := s.sc.FindPerformanceResultsByVersion(s.results[0].info.Version, tr)
	s.Equal(expectedResult, actualResult)
	s.Error(err)
}

func (s *PerfConnectorSuite) TestFindPerformanceResultWithChildren() {
	expectedLineage := s.getLineage(s.results[0].info.ID())
	actualResult, err := s.sc.FindPerformanceResultWithChildren(s.results[0].info.ID(), 5)
	for _, result := range actualResult {
		_, ok := expectedLineage[*result.Name]
		s.True(ok)
	}
	s.NoError(err)

	// Now with tags
	actualResult, err = s.sc.FindPerformanceResultWithChildren(s.results[0].info.ID(), 5, "tag3")
	for _, result := range actualResult {
		_, ok := expectedLineage[*result.Name]
		s.True(ok)
	}
	s.Require().True(len(actualResult) > 1)
	for _, result := range actualResult[1:] {
		foundTag := false
		for _, tag := range result.Info.Tags {
			if tag == "tag3" {
				foundTag = true
				break
			}
		}
		s.True(foundTag)
	}
	s.NoError(err)
}

func (s *PerfConnectorSuite) TestFindPerformanceResultWithChildrenDoesNotExist() {
	var expectedResult []dataModel.APIPerformanceResult
	actualResult, err := s.sc.FindPerformanceResultWithChildren("doesNotExist", 5)
	s.Equal(expectedResult, actualResult)
	s.Error(err)
}

func (s *PerfConnectorSuite) TestFindPerformanceResultWithChildrenDepth() {
	// No depth
	actualResult, err := s.sc.FindPerformanceResultWithChildren(s.results[0].info.ID(), 0)
	s.NoError(err)
	s.Require().Equal(1, len(actualResult))
	s.Equal(s.results[0].info.ID(), *actualResult[0].Name)

	// Direct children
	expectedIds := map[string]bool{}
	for i := 0; i < 3; i++ {
		expectedIds[s.results[i].info.ID()] = true
	}
	actualResult, err = s.sc.FindPerformanceResultWithChildren(s.results[0].info.ID(), 1)
	s.NoError(err)
	s.Require().Equal(3, len(actualResult))
	for _, result := range actualResult {
		_, ok := expectedIds[*result.Name]
		s.Require().True(ok)
		delete(expectedIds, *result.Name)
	}
}
