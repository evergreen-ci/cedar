package model

import dbmodel "github.com/evergreen-ci/cedar/model"

// APIChangePointGroupedByVersionResult describes a paginated list of changepoints
// grouped by the version they each belong to.
type APIChangePointGroupedByVersionResult struct {
	Versions   []APIVersionWithChangePoints `json:"versions"`
	Page       int                          `json:"page"`
	PageSize   int                          `json:"page_size"`
	TotalPages int                          `json:"total_pages"`
}

type APIVersionWithChangePoints struct {
	VersionId    string                       `json:"version_id"`
	ChangePoints []APIChangePointWithPerfData `json:"change_points"`
}

type APIChangePointWithPerfData struct {
	PerfResultId string           `json:"perf_result_id"`
	Project      string           `json:"project"`
	Task         string           `json:"task"`
	Test         string           `json:"test"`
	Variant      string           `json:"variant"`
	Arguments    map[string]int32 `json:"arguments"`
	dbmodel.ChangePoint
}

func CreateAPIChangePointGroupedByVersionResult(getChangePointsGroupedByVersionResult []dbmodel.GetChangePointsGroupedByVersionResult, page, pageSize, totalPages int) *APIChangePointGroupedByVersionResult {
	changePointsGroupedByVersion := make([]APIVersionWithChangePoints, len(getChangePointsGroupedByVersionResult))
	for i, perfResultsWithVersion := range getChangePointsGroupedByVersionResult {
		var apiChangePoints []APIChangePointWithPerfData
		for _, perfResult := range perfResultsWithVersion.PerfResults {
			for _, dbChangePoint := range perfResult.Analysis.ChangePoints {
				changePoint := APIChangePointWithPerfData{
					PerfResultId: perfResult.ID,
					Project:      perfResult.Info.Project,
					Task:         perfResult.Info.TaskName,
					Test:         perfResult.Info.TestName,
					Variant:      perfResult.Info.Variant,
					Arguments:    perfResult.Info.Arguments,
					ChangePoint:  dbChangePoint,
				}
				apiChangePoints = append(apiChangePoints, changePoint)
			}
		}

		changePointsGroupedByVersion[i] = APIVersionWithChangePoints{
			VersionId:    perfResultsWithVersion.VersionID,
			ChangePoints: apiChangePoints,
		}
	}
	return &APIChangePointGroupedByVersionResult{
		Versions:   changePointsGroupedByVersion,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}
}
