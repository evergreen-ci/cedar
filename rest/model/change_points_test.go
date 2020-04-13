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
			VersionId: "version1",
			ChangePoints: []dbmodel.ChangePointWithPerformanceData{
				{
					PerformanceResultSeriesID: dbmodel.PerformanceResultSeriesID{
						Project:     "project1",
						Variant:     "variant1",
						Task:        "task1",
						Test:        "test1",
						ThreadLevel: 10,
					},
					ChangePoint: dbmodel.ChangePoint{
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
					PerfResultId: "perf1",
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

		assert.Equal(t, apiResults.Versions[0].VersionId, testInput[0].VersionId)
		for i, changePoint := range apiResults.Versions[0].ChangePoints {
			inputChangePoint := testInput[0].ChangePoints[i]
			assert.Equal(t, inputChangePoint.ChangePoint, changePoint.ChangePoint)
			assert.Equal(t, inputChangePoint.PerformanceResultSeriesID, changePoint.PerformanceResultSeriesID)
			assert.Equal(t, inputChangePoint.PerfResultId, changePoint.PerfResultId)
		}
	})
}
