package data

import (
	"context"
	"net/http"

	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/gimlet"
	"github.com/pkg/errors"
)

/////////////////////////////
// DBConnector Implementation
/////////////////////////////

// GetHistoricalTestData queries the service backend to retrieve the aggregated
// historical test data that match the given filter.
func (dbc *DBConnector) GetHistoricalTestData(ctx context.Context, f dbModel.HistoricalTestDataFilter) ([]model.APIAggregatedHistoricalTestData, error) {
	data, err := dbModel.GetHistoricalTestData(ctx, dbc.env, f)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "problem fetching historical test data").Error(),
		}
	}

	apiData := make([]model.APIAggregatedHistoricalTestData, len(data))
	for i, d := range data {
		if err = apiData[i].Import(d); err != nil {
			return nil, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    errors.Wrap(err, "corrupt data for historical test data").Error(),
			}
		}
	}

	return apiData, nil
}

///////////////////////////////
// MockConnector Implementation
///////////////////////////////

// GetHistoricalTestData returns the cached historical test data, only
// enforcing the Limit field of the filter.
func (mc *MockConnector) GetHistoricalTestData(ctx context.Context, f dbModel.HistoricalTestDataFilter) ([]model.APIAggregatedHistoricalTestData, error) {
	var data []dbModel.AggregatedHistoricalTestData
	if f.Limit > len(mc.CachedHistoricalTestData) || f.Limit < 1 {
		data = mc.CachedHistoricalTestData
	} else {
		data = mc.CachedHistoricalTestData[:f.Limit]
	}

	apiData := make([]model.APIAggregatedHistoricalTestData, len(data))
	for i, d := range data {
		if err := apiData[i].Import(d); err != nil {
			return nil, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    errors.Wrap(err, "corrupt data for historical test data").Error(),
			}
		}
	}

	return apiData, nil
}
