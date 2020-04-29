package data

import (
	"context"
	"fmt"
	"net/http"

	"github.com/evergreen-ci/cedar/model"
	dataModel "github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/gimlet"
)

/////////////////////////////
// DBConnector Implementation
/////////////////////////////

// GetChangePointsByVersion returns changepoints grouped by version associated with
// the given project. Paginated.
func (dbc *DBConnector) GetChangePointsByVersion(ctx context.Context, projectId string, page, pageSize int, variantRegex, versionRegex, taskRegex, testRegex, measurementRegex string, threadLevels []int) (*dataModel.APIChangePointGroupedByVersionResult, error) {
	totalPages, err := model.GetTotalPagesForChangePointsGroupedByVersion(ctx, dbc.env, projectId, pageSize, variantRegex, versionRegex, taskRegex, testRegex, measurementRegex, threadLevels)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("Error getting total pages of changepoints for project '%s'", projectId),
		}
	}
	changePointsGroupedByVersion, err := model.GetChangePointsGroupedByVersion(ctx, dbc.env, projectId, page, pageSize, variantRegex, versionRegex, taskRegex, testRegex, measurementRegex, threadLevels)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("Error getting changepoints grouped by version for project '%s'", projectId),
		}
	}
	return dataModel.CreateAPIChangePointGroupedByVersionResult(changePointsGroupedByVersion, page, pageSize, totalPages), nil
}

///////////////////////////////
// MockConnector Implementation
///////////////////////////////

// GetChangePointsByVersion returns changepoints grouped by version associated with
// the given project. Paginated.
func (mc *MockConnector) GetChangePointsByVersion(ctx context.Context, projectId string, page, pageSize int, variantRegex, versionRegex, taskRegex, testRegex, measurementRegex string, threadLevelRegex []int) (*dataModel.APIChangePointGroupedByVersionResult, error) {
	return &dataModel.APIChangePointGroupedByVersionResult{
		Versions:   nil,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: 0,
	}, nil
}
