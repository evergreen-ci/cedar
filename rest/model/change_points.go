package model

import dbmodel "github.com/evergreen-ci/cedar/model"

// APIChangePointGroupedByVersionResult describes a paginated list of changepoints
// grouped by the version they each belong to.
type APIChangePointGroupedByVersionResult struct {
	Versions   []APIChangePointsWithVersion `json:"versions"`
	Page       int                          `json:"page"`
	PageSize   int                          `json:"page_size"`
	TotalPages int                          `json:"total_pages"`
}

type APIChangePointsWithVersion struct {
	VersionId    string                       `json:"version_id"`
	ChangePoints []APIChangePointWithPerfData `json:"change_points"`
}

type APIChangePointWithPerfData struct {
	PerfResultId string `bson:"perf_result_id" json:"perf_result_id"`
	Project      string `bson:"project" json:"project"`
	Task         string `bson:"task" json:"task"`
	Test         string `bson:"test" json:"test"`
	Variant      string `bson:"variant" json:"variant"`
	ThreadLevel  int32  `bson:"thread_level" json:"thread_level"`
	dbmodel.ChangePoint
}

func CreateAPIChangePointGroupedByVersionResult(getChangePointsGroupedByVersionResult []dbmodel.GetChangePointsGroupedByVersionResult, page, pageSize, totalPages int) *APIChangePointGroupedByVersionResult {
	changePointsGroupedByVersion := make([]APIChangePointsWithVersion, len(getChangePointsGroupedByVersionResult))
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
					ThreadLevel:  perfResult.Info.Arguments["thread_level"],
					ChangePoint:  dbChangePoint,
				}
				apiChangePoints = append(apiChangePoints, changePoint)
			}
		}

		changePointsGroupedByVersion[i] = APIChangePointsWithVersion{
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
