package data

import (
	"context"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/suite"
)

func (s *ChangePointConnectorSuite) createPerformanceResultsWithChangePoints() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.env = cedar.GetEnvironment()
	s.Require().NotNil(s.env)
	db := s.env.GetDB()
	db.Drop(s.ctx)
	s.Require().NotNil(db)

	perfResults := []*model.PerformanceResult{
		{
			ID: "perfResult1",
			Info: model.PerformanceResultInfo{
				Project:  "project1",
				Version:  "version1",
				Variant:  "variant1",
				TaskName: "task1",
				TestName: "test1",
				Arguments: map[string]int32{
					"thread_level": 10,
				},
				Order: 1,
			},
			Analysis: model.PerfAnalysis{
				ChangePoints: []model.ChangePoint{
					{
						Index:        1,
						Measurement:  "measurement1",
						CalculatedOn: time.Now(),
						Algorithm: model.AlgorithmInfo{
							Name:    "edvisive",
							Version: 10,
							Options: []model.AlgorithmOption{
								{
									Name:  "option1",
									Value: 10,
								},
							},
						},
						Triage: model.TriageInfo{
							TriagedOn: time.Now(),
							Status:    "untriaged",
						},
					},
					{
						Index:        2,
						Measurement:  "measurement2",
						CalculatedOn: time.Now().Add(1 * time.Minute),
						Algorithm: model.AlgorithmInfo{
							Name:    "default",
							Version: 10,
							Options: []model.AlgorithmOption{
								{
									Name:  "option1",
									Value: 10,
								},
							},
						},
						Triage: model.TriageInfo{
							TriagedOn: time.Now(),
							Status:    "triaged",
						},
					},
				},
				ProcessedAt: time.Now(),
			},
		},
		{
			ID: "perfResult2",
			Info: model.PerformanceResultInfo{
				Project: "project2",
				Order:   2,
			},
			Analysis: model.PerfAnalysis{
				ChangePoints: []model.ChangePoint{
					{
						Index:       1,
						Measurement: "measurement3",
					},
				},
				ProcessedAt: time.Now(),
			},
		},
		{
			ID: "perfResult3",
			Info: model.PerformanceResultInfo{
				Project:  "project1",
				Version:  "version3",
				Variant:  "variant3",
				TaskName: "task3",
				TestName: "test3",
				Arguments: map[string]int32{
					"thread_level": 15,
				},
				Order: 3,
			},
			Analysis: model.PerfAnalysis{
				ChangePoints: []model.ChangePoint{
					{
						Index:        3,
						Measurement:  "measurement1",
						CalculatedOn: time.Now().Add(5 * time.Minute),
						Algorithm: model.AlgorithmInfo{
							Name:    "edvisive",
							Version: 15,
							Options: []model.AlgorithmOption{
								{
									Name:  "option1",
									Value: 15,
								},
							},
						},
						Triage: model.TriageInfo{
							TriagedOn: time.Now(),
							Status:    "triaged",
						},
					},
				},
				ProcessedAt: time.Now(),
			},
		},
	}

	for _, result := range perfResults {
		populatedPerfResult := model.CreatePerformanceResult(result.Info, nil, nil)
		populatedPerfResult.Analysis = result.Analysis
		populatedPerfResult.ID = result.ID
		populatedPerfResult.Setup(s.env)
		err := populatedPerfResult.SaveNew(ctx)
		if err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

type ChangePointConnectorSuite struct {
	ctx    context.Context
	cancel context.CancelFunc
	sc     Connector
	env    cedar.Environment

	suite.Suite
}

func TestChangePointConnectorSuiteDB(t *testing.T) {
	c := new(ChangePointConnectorSuite)
	c.setup()
	c.sc = CreateNewDBConnector(c.env)
	suite.Run(t, c)
}

func (s *ChangePointConnectorSuite) setup() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.env = cedar.GetEnvironment()
	err := s.createPerformanceResultsWithChangePoints()
	s.Require().NoError(err)
}

func (s *ChangePointConnectorSuite) TearDownSuite() {
	defer s.cancel()
	err := tearDownEnv(s.env)
	s.Require().NoError(err)
}

func (s *ChangePointConnectorSuite) TestFilteringByVariant() {
	page := 0
	pageSize := 100
	projectId := "project1"
	result, err := s.sc.GetChangePointsByVersion(s.ctx, projectId, page, pageSize, "variant1", "", "", "", "", nil)
	s.NoError(err)
	totalChangePoints := 0
	for _, versionWithChangePoints := range result.Versions {
		totalChangePoints += len(versionWithChangePoints.ChangePoints)
	}
	s.Require().Equal(2, totalChangePoints)

	result, err = s.sc.GetChangePointsByVersion(s.ctx, projectId, page, pageSize, "variant3", "", "", "", "", nil)
	s.NoError(err)
	totalChangePoints = 0
	for _, versionWithChangePoints := range result.Versions {
		totalChangePoints += len(versionWithChangePoints.ChangePoints)
	}
	s.Require().Equal(1, totalChangePoints)
}

func (s *ChangePointConnectorSuite) TestFilteringByVersion() {
	page := 0
	pageSize := 100
	projectId := "project1"
	result, err := s.sc.GetChangePointsByVersion(s.ctx, projectId, page, pageSize, "", "version1", "", "", "", nil)
	s.NoError(err)
	totalChangePoints := 0
	for _, versionWithChangePoints := range result.Versions {
		totalChangePoints += len(versionWithChangePoints.ChangePoints)
	}
	s.Require().Equal(2, totalChangePoints)

	result, err = s.sc.GetChangePointsByVersion(s.ctx, projectId, page, pageSize, "", "version3", "", "", "", nil)
	s.NoError(err)
	totalChangePoints = 0
	for _, versionWithChangePoints := range result.Versions {
		totalChangePoints += len(versionWithChangePoints.ChangePoints)
	}
	s.Require().Equal(1, totalChangePoints)
}

func (s *ChangePointConnectorSuite) TestFilteringByTask() {
	page := 0
	pageSize := 100
	projectId := "project1"
	result, err := s.sc.GetChangePointsByVersion(s.ctx, projectId, page, pageSize, "", "", "task1", "", "", nil)
	s.NoError(err)
	totalChangePoints := 0
	for _, versionWithChangePoints := range result.Versions {
		totalChangePoints += len(versionWithChangePoints.ChangePoints)
	}
	s.Require().Equal(2, totalChangePoints)

	result, err = s.sc.GetChangePointsByVersion(s.ctx, projectId, page, pageSize, "", "", "task3", "", "", nil)
	s.NoError(err)
	totalChangePoints = 0
	for _, versionWithChangePoints := range result.Versions {
		totalChangePoints += len(versionWithChangePoints.ChangePoints)
	}
	s.Require().Equal(1, totalChangePoints)
}

func (s *ChangePointConnectorSuite) TestFilteringByTest() {
	page := 0
	pageSize := 100
	projectId := "project1"
	result, err := s.sc.GetChangePointsByVersion(s.ctx, projectId, page, pageSize, "", "", "", "test1", "", nil)
	s.NoError(err)
	totalChangePoints := 0
	for _, versionWithChangePoints := range result.Versions {
		totalChangePoints += len(versionWithChangePoints.ChangePoints)
	}
	s.Require().Equal(2, totalChangePoints)

	result, err = s.sc.GetChangePointsByVersion(s.ctx, projectId, page, pageSize, "", "", "", "test3", "", nil)
	s.NoError(err)
	totalChangePoints = 0
	for _, versionWithChangePoints := range result.Versions {
		totalChangePoints += len(versionWithChangePoints.ChangePoints)
	}
	s.Require().Equal(1, totalChangePoints)
}

func (s *ChangePointConnectorSuite) TestFilteringByMeasurement() {
	page := 0
	pageSize := 100
	projectId := "project1"
	result, err := s.sc.GetChangePointsByVersion(s.ctx, projectId, page, pageSize, "", "", "", "", "measurement", nil)
	s.NoError(err)
	totalChangePoints := 0
	for _, versionWithChangePoints := range result.Versions {
		totalChangePoints += len(versionWithChangePoints.ChangePoints)
	}
	s.Require().Equal(3, totalChangePoints)

	result, err = s.sc.GetChangePointsByVersion(s.ctx, projectId, page, pageSize, "", "", "", "", "measurement1", nil)
	s.NoError(err)
	totalChangePoints = 0
	for _, versionWithChangePoints := range result.Versions {
		totalChangePoints += len(versionWithChangePoints.ChangePoints)
	}
	s.Require().Equal(2, totalChangePoints)

	result, err = s.sc.GetChangePointsByVersion(s.ctx, projectId, page, pageSize, "", "", "", "", "measurement2", nil)
	s.NoError(err)
	totalChangePoints = 0
	for _, versionWithChangePoints := range result.Versions {
		totalChangePoints += len(versionWithChangePoints.ChangePoints)
	}
	s.Require().Equal(1, totalChangePoints)
}

func (s *ChangePointConnectorSuite) TestFilteringByThreadLevel() {
	page := 0
	pageSize := 100
	projectId := "project1"
	result, err := s.sc.GetChangePointsByVersion(s.ctx, projectId, page, pageSize, "", "", "", "", "", []int{10,15})
	s.NoError(err)
	totalChangePoints := 0
	for _, versionWithChangePoints := range result.Versions {
		totalChangePoints += len(versionWithChangePoints.ChangePoints)
	}
	s.Require().Equal(3, totalChangePoints)

	result, err = s.sc.GetChangePointsByVersion(s.ctx, projectId, page, pageSize, "", "", "", "", "", []int{10})
	s.NoError(err)
	totalChangePoints = 0
	for _, versionWithChangePoints := range result.Versions {
		totalChangePoints += len(versionWithChangePoints.ChangePoints)
	}
	s.Require().Equal(2, totalChangePoints)

	result, err = s.sc.GetChangePointsByVersion(s.ctx, projectId, page, pageSize, "", "", "", "", "", []int{15})
	s.NoError(err)
	totalChangePoints = 0
	for _, versionWithChangePoints := range result.Versions {
		totalChangePoints += len(versionWithChangePoints.ChangePoints)
	}
	s.Require().Equal(1, totalChangePoints)
}

func (s *ChangePointConnectorSuite) TestFilteringByEverything() {
	page := 0
	pageSize := 100
	projectId := "project1"
	result, err := s.sc.GetChangePointsByVersion(s.ctx, projectId, page, pageSize, "variant1", "version1", "task1", "test1", "measurement1", []int{10})
	s.NoError(err)
	totalChangePoints := 0
	for _, versionWithChangePoints := range result.Versions {
		totalChangePoints += len(versionWithChangePoints.ChangePoints)
	}
	s.Require().Equal(1, totalChangePoints)
}

func (s *ChangePointConnectorSuite) TestFilteringByEverythingNoResults() {
	page := 0
	pageSize := 100
	projectId := "project1"
	result, err := s.sc.GetChangePointsByVersion(s.ctx, projectId, page, pageSize, "variant1", "version1", "task1", "test1", "measurement1", []int{2})
	s.NoError(err)
	totalChangePoints := 0
	for _, versionWithChangePoints := range result.Versions {
		totalChangePoints += len(versionWithChangePoints.ChangePoints)
	}
	s.Require().Equal(0, totalChangePoints)
}

func (s *ChangePointConnectorSuite) TestGetChangePointsByVersion() {
	page := 0
	pageSize := 100
	projectId := "project1"
	result, err := s.sc.GetChangePointsByVersion(s.ctx, projectId, page, pageSize, "", "", "", "", "", nil)
	if err != nil {
		print(err)
	}
	s.NoError(err)
	s.Require().Equal(page, result.Page)
	s.Require().Equal(pageSize, result.PageSize)
	s.Require().Equal(1, result.TotalPages)
	s.Require().Equal(2, len(result.Versions))
	totalChangePoints := 0
	for _, versionWithChangePoints := range result.Versions {
		for _, changePoint := range versionWithChangePoints.ChangePoints {
			s.Require().Equal(projectId, changePoint.Project)
			totalChangePoints += 1
		}
	}
	s.Require().Equal(3, totalChangePoints)
}

func (s *ChangePointConnectorSuite) TestGetChangePointsByVersionPaging() {
	page := 0
	pageSize := 1
	projectId := "project1"
	result, err := s.sc.GetChangePointsByVersion(s.ctx, projectId, page, pageSize, "", "", "", "", "", nil)
	if err != nil {
		print(err)
	}
	s.NoError(err)
	s.Require().Equal(page, result.Page)
	s.Require().Equal(pageSize, result.PageSize)
	s.Require().Equal(2, result.TotalPages)
	seenPerfResults := []int{result.Versions[0].ChangePoints[0].Index}
	for i := result.Page + 1; i < result.TotalPages; i++ {
		result, err := s.sc.GetChangePointsByVersion(s.ctx, projectId, i, pageSize, "", "", "", "", "", nil)
		if err != nil {
			print(err)
		}
		s.NoError(err)
		s.Require().Equal(i, result.Page)
		s.Require().Equal(pageSize, result.PageSize)
		s.Require().Equal(2, result.TotalPages)
		s.Require().NotContains(seenPerfResults, result.Versions[0].ChangePoints[0].Index)
		seenPerfResults = append(seenPerfResults, result.Versions[0].ChangePoints[0].Index)
	}
}
