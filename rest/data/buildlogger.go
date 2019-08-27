package data

import (
	"context"
	"fmt"
	"net/http"

	"github.com/evergreen-ci/cedar/model"
	dataModel "github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/gimlet"
)

// FindLogById queries the database to find the buildlogger log with the given
// id.
func (dbc *DBConnector) FindLogById(ctx context.Context, id string) (*dataModel.APILog, error) {
	log := model.Log{ID: id}
	log.Setup(dbc.env)

	if err := log.Find(ctx); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("log with id '%s' not found", id),
		}
	}

	apiLog := &dataModel.APILog{}
	if err := apiLog.Import(log); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("corrupt data"),
		}
	}

	return apiLog, nil
}

///////////////////////////////
// MockConnector Implementation
///////////////////////////////

// FindLogById queries the mock cache to find the buildlogger log with the
// given id.
func (mc MockConnector) FindLogById(_ context.Context, id string) (*dataModel.APILog, error) {
	log, ok := mc.CachedLogs[id]
	if !ok {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("log with id '%s' not found", id),
		}
	}

	apiLog := &dataModel.APILog{}
	if err := apiLog.Import(log); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("corrupt data"),
		}
	}

	return apiLog, nil
}
