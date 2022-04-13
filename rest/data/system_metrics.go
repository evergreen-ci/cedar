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

var softSizeLimit = 10 * 1024 * 1024

/////////////////////////////
// DBConnector Implementation
/////////////////////////////

func (dbc *DBConnector) FindSystemMetricsByType(ctx context.Context, findOpts dbModel.SystemMetricsFindOptions, downloadOpts dbModel.SystemMetricsDownloadOptions) ([]byte, int, error) {
	sm := &dbModel.SystemMetrics{}
	sm.Setup(dbc.env)
	if err := sm.FindByTaskID(ctx, findOpts); db.ResultsNotFound(err) {
		return nil, 0, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("system metrics for task ID '%s' not found", findOpts.TaskID),
		}
	} else if err != nil {
		return nil, 0, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "retrieving system metrics data for task ID '%s'", findOpts.TaskID).Error(),
		}
	}

	// check that the metric is valid so we can return the appropriate
	// error code.
	_, ok := sm.Artifact.MetricChunks[downloadOpts.MetricType]
	if !ok {
		return nil, 0, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("metric type '%s' for task ID '%s' not found", downloadOpts.MetricType, findOpts.TaskID),
		}
	}

	data, idx, err := sm.DownloadWithPagination(ctx, downloadOpts)
	if err != nil {
		return nil, 0, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "downloading raw system metrics data for task ID '%s'", findOpts.TaskID).Error(),
		}
	}
	return data, idx, nil
}

///////////////////////////////
// MockConnector Implementation
///////////////////////////////

func (mc *MockConnector) FindSystemMetricsByType(ctx context.Context, findOpts dbModel.SystemMetricsFindOptions, downloadOpts dbModel.SystemMetricsDownloadOptions) ([]byte, int, error) {
	var sm *dbModel.SystemMetrics
	for key := range mc.CachedSystemMetrics {
		val := mc.CachedSystemMetrics[key]
		if findOpts.EmptyExecution {
			if val.Info.TaskID == findOpts.TaskID && (sm == nil || val.Info.Execution > sm.Info.Execution) {
				sm = &val
			}
		} else if val.Info.TaskID == findOpts.TaskID && val.Info.Execution == findOpts.Execution {
			sm = &val
			break
		}
	}
	if sm == nil {
		return nil, 0, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("system metrics for task ID '%s' not found", findOpts.TaskID),
		}
	}

	// check that the metric is valid so we can return the appropriate
	// error code.
	chunks, ok := sm.Artifact.MetricChunks[downloadOpts.MetricType]
	if !ok {
		return nil, 0, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("metric type '%s' for task ID '%s' not found", downloadOpts.MetricType, findOpts.TaskID),
		}
	}

	if downloadOpts.StartIndex >= len(chunks.Chunks) {
		return nil, downloadOpts.StartIndex, nil
	}

	bucketOpts := pail.LocalOptions{
		Path:   mc.Bucket,
		Prefix: sm.Artifact.Prefix,
	}
	bucket, err := pail.NewLocalBucket(bucketOpts)
	if err != nil {
		return nil, 0, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "creating bucket").Error(),
		}
	}

	var (
		totalSize int
		data      []byte
	)
	idx := downloadOpts.StartIndex
	for _, key := range chunks.Chunks[downloadOpts.StartIndex:] {
		r, err := bucket.Get(ctx, key)
		if err != nil {
			return nil, 0, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    errors.Wrap(err, "fetching data").Error(),
			}
		}

		catcher := grip.NewBasicCatcher()
		chunkData, err := ioutil.ReadAll(r)
		catcher.Wrap(err, "reading data")
		catcher.Wrap(r.Close(), "closing read closer")
		if catcher.HasErrors() {
			return nil, 0, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    catcher.Resolve().Error(),
			}
		}

		idx += 1
		data = append(data, chunkData...)
		totalSize += len(chunkData)
		if downloadOpts.PageSize > 0 && totalSize >= downloadOpts.PageSize {
			break
		}
	}

	return data, idx, nil
}
