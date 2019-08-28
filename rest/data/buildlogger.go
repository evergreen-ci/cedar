package data

import (
	"context"
	"fmt"
	"net/http"

	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/gimlet"
	"github.com/pkg/errors"
)

/////////////////////////////
// DBConnector Implementation
/////////////////////////////

// FindLogById queries the database to find the buildlogger log with the given
// id returning the metadata and a LogIterator with the corresponding time
// range.
func (dbc *DBConnector) FindLogById(ctx context.Context, id string, tr util.TimeRange) (*model.APILog, dbModel.LogIterator, error) {
	log := dbModel.Log{ID: id}
	log.Setup(dbc.env)
	if err := log.Find(ctx); err != nil {
		return nil, nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("log with id '%s' not found", id),
		}
	}

	apiLog := &model.APILog{}
	if err := apiLog.Import(log); err != nil {
		return nil, nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    "corrupt data",
		}
	}

	log.Setup(dbc.env)
	it, err := log.Download(ctx, tr)
	if err != nil {
		return nil, nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("%s", errors.Wrap(err, "problem downloading log")),
		}
	}

	return apiLog, it, nil
}

///////////////////////////////
// MockConnector Implementation
///////////////////////////////

// FindLogById queries the mock cache to find the buildlogger log with the
// given id returning the metadata and a LogIterator with the corresponding
// time range.
func (mc MockConnector) FindLogById(ctx context.Context, id string, tr util.TimeRange) (*model.APILog, dbModel.LogIterator, error) {
	log, ok := mc.CachedLogs[id]
	if !ok {
		return nil, nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("log with id '%s' not found", id),
		}
	}

	apiLog := &model.APILog{}
	if err := apiLog.Import(log); err != nil {
		return nil, nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    "corrupt data",
		}
	}

	log.Setup(mc.env)
	it, err := log.Download(ctx, tr)
	if err != nil {
		return nil, nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("%s", errors.Wrap(err, "problem downloading log")),
		}
	}

	return apiLog, it, nil
}
