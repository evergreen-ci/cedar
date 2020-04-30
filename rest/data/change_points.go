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
func (dbc *DBConnector) GetChangePointsByVersion(ctx context.Context, args GetChangePointsGroupedByVersionArgs) (*dataModel.APIChangePointGroupedByVersionResult, error) {
	totalPages, err := model.GetTotalPagesForChangePointsGroupedByVersion(ctx, dbc.env, args)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("Error getting total pages of changepoints for project '%s'", args.ProjectId),
		}
	}
	changePointsGroupedByVersion, err := model.GetChangePointsGroupedByVersion(ctx, dbc.env, args)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("Error getting changepoints grouped by version for project '%s'", args.ProjectId),
		}
	}
	return dataModel.CreateAPIChangePointGroupedByVersionResult(changePointsGroupedByVersion, args.Page, args.PageSize, totalPages), nil
}

///////////////////////////////
// MockConnector Implementation
///////////////////////////////

// GetChangePointsByVersion returns changepoints grouped by version associated with
// the given project. Paginated.
func (mc *MockConnector) GetChangePointsByVersion(ctx context.Context, args GetChangePointsGroupedByVersionArgs) (*dataModel.APIChangePointGroupedByVersionResult, error) {
	return &dataModel.APIChangePointGroupedByVersionResult{
		Versions:   nil,
		Page:       args.Page,
		PageSize:   args.PageSize,
		TotalPages: 0,
	}, nil
}
