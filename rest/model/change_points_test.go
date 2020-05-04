package model

import (
	"testing"
	"time"

	dbmodel "github.com/evergreen-ci/cedar/model"
	"github.com/stretchr/testify/assert"
)

func TestCreateAPIChangePointGroupedByVersionResult(t *testing.T) {
	testInput := []dbmodel.GetChangePointsGroupedByVersionResult{
		{
			VersionID: "version1",
			PerfResults: []dbmodel.PerformanceResult{
				{
					Info: dbmodel.PerformanceResultInfo{
						Project:   "project1",
						Variant:   "variant1",
						TaskName:  "task1",
						TestName:  "test1",
						Arguments: map[string]int32{"thread_level": 10},
					},
					Analysis: dbmodel.PerfAnalysis{
						ChangePoints: []dbmodel.ChangePoint{
							{
								Index:        10,
								Measurement:  "measurement1",
								CalculatedOn: time.Now(),
								Algorithm: dbmodel.AlgorithmInfo{
									Name:    "algo1",
									Version: 31,
								},
								Triage: dbmodel.TriageInfo{
									TriagedOn: time.Now(),
									Status:    "triaged",
								},
							},
						},
					},
					ID: "perf1",
				},
			},
		},
	}
	t.Run("CreateAPIChangePointGroupedByVersionResult correctly", func(t *testing.T) {
		page := 0
		totalPages := 100
		pageSize := 7
		apiResults := CreateAPIChangePointGroupedByVersionResult(testInput, page, pageSize, totalPages)
		assert.Equal(t, page, apiResults.Page)
		assert.Equal(t, pageSize, apiResults.PageSize)
		assert.Equal(t, totalPages, apiResults.TotalPages)

		assert.Equal(t, apiResults.Versions[0].VersionId, testInput[0].VersionID)
		for i, changePoint := range apiResults.Versions[0].ChangePoints {
			perfResult := testInput[0].PerfResults[i]
			assert.Equal(t, perfResult.Analysis.ChangePoints[0], changePoint.ChangePoint)
			assert.Equal(t, perfResult.Info.Project, changePoint.Project)
			assert.Equal(t, perfResult.Info.Variant, changePoint.Variant)
			assert.Equal(t, perfResult.Info.TaskName, changePoint.Task)
			assert.Equal(t, perfResult.Info.TestName, changePoint.Test)
			assert.Equal(t, perfResult.Info.Arguments, changePoint.Arguments)
			assert.Equal(t, perfResult.ID, changePoint.PerfResultId)
		}
	})
}
