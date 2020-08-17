package data

import (
	"context"
	"fmt"
	"io"
	"net/http"

	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/anser/db"
	"github.com/pkg/errors"
)

/////////////////////////////
// DBConnector Implementation
/////////////////////////////

func (dbc *DBConnector) FindSystemMetricsByType(ctx context.Context, metricType string, opts dbModel.SystemMetricsFindOptions) (io.ReadCloser, error) {
	sm := &dbModel.SystemMetrics{}
	sm.Setup(dbc.env)
	if err := sm.FindByTaskID(ctx, opts); db.ResultsNotFound(err) {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("system metrics for task id '%s' not found", opts.TaskID),
		}
	} else if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "problem retrieving system metrics data for task id '%s'", opts.TaskID).Error(),
		}
	}

	// check that the metric is valid so we can return the appropriate
	// error code.
	_, ok := sm.Artifact.MetricChunks[metricType]
	if !ok {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("metric type '%s' for task id '%s' not found", metricType, opts.TaskID),
		}
	}
	r, err := sm.Download(ctx, metricType)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "problem downloading raw system metrics data for task id '%s'", opts.TaskID).Error(),
		}
	}

	return r, nil
}

///////////////////////////////
// MockConnector Implementation
///////////////////////////////

func (mc *MockConnector) FindSystemMetricsByType(ctx context.Context, metricType string, opts dbModel.SystemMetricsFindOptions) (io.ReadCloser, error) {
	var sm *dbModel.SystemMetrics
	for key := range mc.CachedSystemMetrics {
		val := mc.CachedSystemMetrics[key]
		if opts.EmptyExecution {
			if val.Info.TaskID == opts.TaskID && (sm == nil || val.Info.Execution > sm.Info.Execution) {
				sm = &val
			}
		} else if val.Info.TaskID == opts.TaskID && val.Info.Execution == opts.Execution {
			sm = &val
			break
		}
	}
	if sm == nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("system metrics for task id '%s' not found", opts.TaskID),
		}
	}

	// check that the metric is valid so we can return the appropriate
	// error code.
	_, ok := sm.Artifact.MetricChunks[metricType]
	if !ok {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("metric type '%s' for task id '%s' not found", metricType, opts.TaskID),
		}
	}
	sm.Setup(mc.env)
	r, err := sm.Download(ctx, metricType)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "problem downloading raw system metrics data for task id '%s'", opts.TaskID).Error(),
		}
	}

	return r, nil
}
