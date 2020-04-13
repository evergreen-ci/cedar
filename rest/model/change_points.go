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
	dbmodel.ChangePoint
	dbmodel.PerformanceResultSeriesID
}

func CreateAPIChangePointGroupedByVersionResult(changePointsWithVersions []dbmodel.GetChangePointsGroupedByVersionResult, page, pageSize, totalPages int) *APIChangePointGroupedByVersionResult {
	apiChangePoints := make([]APIChangePointsWithVersion, len(changePointsWithVersions))
	for i, changePointWithVersion := range changePointsWithVersions {
		changePoints := make([]APIChangePointWithPerfData, len(changePointWithVersion.ChangePoints))
		for j, dbChangePoint := range changePointWithVersion.ChangePoints {
			changePoints[j] = APIChangePointWithPerfData{
				PerfResultId:              dbChangePoint.PerfResultId,
				ChangePoint:               dbChangePoint.ChangePoint,
				PerformanceResultSeriesID: dbChangePoint.PerformanceResultSeriesID,
			}
		}
		apiChangePoints[i] = APIChangePointsWithVersion{
			VersionId:    changePointWithVersion.VersionId,
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
