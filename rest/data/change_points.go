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

// GetChangePointsByVersions returns changepoints grouped by version associated with
// the given project. Paginated.
func (dbc *DBConnector) GetChangePointsByVersions(ctx context.Context, projectId string, page, pageSize int) (*dataModel.APIChangePointGroupedByVersionResult, error) {
	totalPages, err := model.GetTotalPagesForChangePointsGroupedByVersion(ctx, dbc.env, projectId, pageSize)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("Error getting total pages of changepoints for project '%s'", projectId),
		}
	}
	changePointsGroupedByVersion, err := model.GetChangePointsGroupedByVersion(ctx, dbc.env, projectId, page, pageSize)
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

// GetChangePointsByVersions returns changepoints grouped by version associated with
// the given project. Paginated.
func (mc *MockConnector) GetChangePointsByVersions(ctx context.Context, projectId string, page, pageSize int) (*dataModel.APIChangePointGroupedByVersionResult, error) {
	return nil, nil
}
