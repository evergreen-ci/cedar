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
	PerfResultId string `json:"perf_result_id"`
	Project      string `json:"project"`
	Task         string `json:"task"`
	Test         string `json:"test"`
	Variant      string `json:"variant"`
	ThreadLevel  int32  `json:"thread_level"`
	dbmodel.ChangePoint
}

func CreateAPIChangePointGroupedByVersionResult(changePointsWithVersions []dbmodel.GetChangePointsGroupedByVersionResult, page, pageSize, totalPages int) *APIChangePointGroupedByVersionResult {
	apiChangePoints := make([]APIChangePointsWithVersion, len(changePointsWithVersions))
	for i, changePointsWithVersion := range changePointsWithVersions {
		changePoints := make([]APIChangePointWithPerfData, len(changePointsWithVersion.ChangePoints))
		for j, dbChangePoint := range changePointsWithVersion.ChangePoints {
			changePoints[j] = APIChangePointWithPerfData{
				PerfResultId: dbChangePoint.PerfResultID,
				ChangePoint:  dbChangePoint.ChangePoint,
				Project:      dbChangePoint.Info.Project,
				Task:         dbChangePoint.Info.TaskName,
				Test:         dbChangePoint.Info.TestName,
				Variant:      dbChangePoint.Info.Variant,
				ThreadLevel:  dbChangePoint.Info.Arguments["thread_level"],
			}
		}
		apiChangePoints[i] = APIChangePointsWithVersion{
			VersionId:    changePointsWithVersion.VersionID,
			ChangePoints: changePoints,
		}
	}
	return &APIChangePointGroupedByVersionResult{
		Versions:   apiChangePoints,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}
}
