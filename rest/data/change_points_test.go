package data

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/suite"
)

func (s *ChangePointConnectorSuite) createPerformanceResultsWithChangePoints(env cedar.Environment) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.env = cedar.GetEnvironment()
	s.Require().NotNil(s.env)
	db := s.env.GetDB()
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
						Index:        1,
						Measurement:  "measurement2",
						CalculatedOn: time.Now(),
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
			},
			Analysis: model.PerfAnalysis{
				ChangePoints: []model.ChangePoint{
					{
						Index:        10,
						Measurement:  "measurement1",
						CalculatedOn: time.Now(),
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
	idMap  map[string]model.PerformanceResult

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
	err := s.createPerformanceResultsWithChangePoints(s.env)
	s.Require().NoError(err)
}

func (s *ChangePointConnectorSuite) TearDownSuite() {
	defer s.cancel()
	err := tearDownEnv(s.env)
	s.Require().NoError(err)
}

func (s *ChangePointConnectorSuite) TestGetChangePointsByVersions() {
	page := 0
	pageSize := 100
	projectId := "project1"
	result, err := s.sc.GetChangePointsByVersions(s.ctx, projectId, page, pageSize)
	if err != nil {
		print(err)
	}
	s.NoError(err)
	s.Require().Equal(result.Page, page)
	s.Require().Equal(result.PageSize, pageSize)
	s.Require().Equal(result.TotalPages, 1)
	s.Require().Equal(len(result.Versions), 2)
	totalChangePoints := 0
	for _, versionWithChangePoints := range result.Versions {
		for _, changePoint := range versionWithChangePoints.ChangePoints {
			s.Require().Equal(changePoint.Project, projectId)
			totalChangePoints += 1
		}
	}
	s.Require().Equal(totalChangePoints, 3)
	bolB, _ := json.Marshal(result)
	fmt.Println(string(bolB))
}
