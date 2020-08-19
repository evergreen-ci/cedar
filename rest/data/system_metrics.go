package data

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/gimlet"
	"github.com/evergreen-ci/pail"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

/////////////////////////////
// DBConnector Implementation
/////////////////////////////

func (dbc *DBConnector) FindSystemMetricsByType(ctx context.Context, metricType string, opts dbModel.SystemMetricsFindOptions) ([]byte, error) {
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

	catcher := grip.NewBasicCatcher()
	data, err := ioutil.ReadAll(r)
	catcher.Add(errors.Wrap(err, "problem reading data"))
	catcher.Add(errors.Wrap(r.Close(), "problem closing read closer"))
	return data, catcher.Resolve()
}

///////////////////////////////
// MockConnector Implementation
///////////////////////////////

func (mc *MockConnector) FindSystemMetricsByType(ctx context.Context, metricType string, opts dbModel.SystemMetricsFindOptions) ([]byte, error) {
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
	chunks, ok := sm.Artifact.MetricChunks[metricType]
	if !ok {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("metric type '%s' for task id '%s' not found", metricType, opts.TaskID),
		}
	}

	bucketOpts := pail.LocalOptions{
		Path:   mc.Bucket,
		Prefix: sm.Artifact.Prefix,
	}
	bucket, err := pail.NewLocalBucket(bucketOpts)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("%s", errors.Wrap(err, "problem creating bucket")),
		}
	}

	catcher := grip.NewBasicCatcher()
	r := dbModel.NewSystemMetricsReadCloser(ctx, bucket, chunks, 2)
	data, err := ioutil.ReadAll(r)
	catcher.Add(errors.Wrap(err, "problem reading data"))
	catcher.Add(errors.Wrap(r.Close(), "problem closing read closer"))
	return data, catcher.Resolve()
}
