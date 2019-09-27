package data

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"

	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/gimlet"
	"github.com/evergreen-ci/pail"
	"github.com/mongodb/anser/db"
	"github.com/pkg/errors"
)

/////////////////////////////
// DBConnector Implementation
/////////////////////////////

// FindLogByID queries the database to find the buildlogger log with the given
// id returning a LogIterator with the corresponding time range.
func (dbc *DBConnector) FindLogByID(ctx context.Context, id string, tr util.TimeRange) (dbModel.LogIterator, error) {
	log := dbModel.Log{ID: id}
	log.Setup(dbc.env)
	if err := log.Find(ctx); db.ResultsNotFound(err) {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("log with id '%s' not found", id),
		}
	} else if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("database error"),
		}
	}

	log.Setup(dbc.env)
	it, err := log.Download(ctx, tr, false)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("%s", errors.Wrap(err, "problem downloading log")),
		}
	}

	return it, nil
}

// FindLogMetadataByID queries the database to find the buildlogger log with
// the given id returning its metadata only.
func (dbc *DBConnector) FindLogMetadataByID(ctx context.Context, id string) (*model.APILog, error) {
	log := dbModel.Log{ID: id}
	log.Setup(dbc.env)
	if err := log.Find(ctx); db.ResultsNotFound(err) {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("log with id '%s' not found", id),
		}
	} else if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("database error"),
		}
	}

	apiLog := &model.APILog{}
	if err := apiLog.Import(log); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    "corrupt data",
		}
	}

	return apiLog, nil
}

// FindLogsByTaskID queries the database to find the buildlogger logs with the
// given task id and optional tags, returning the merged logs via a LogIterator
// with the corresponding time range.
func (dbc *DBConnector) FindLogsByTaskID(ctx context.Context, taskID string, tr util.TimeRange, tags ...string) (dbModel.LogIterator, error) {
	opts := dbModel.LogFindOptions{
		TimeRange: tr,
		Info: dbModel.LogInfo{
			TaskID: taskID,
			Tags:   tags,
		},
	}
	logs := dbModel.Logs{}
	logs.Setup(dbc.env)
	if err := logs.Find(ctx, opts); db.ResultsNotFound(err) {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("logs with task id '%s' not found", taskID),
		}
	} else if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("database error"),
		}
	}

	logs.Setup(dbc.env)
	it, err := logs.Merge(ctx, false)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("%s", errors.Wrap(err, "problem downloading log")),
		}
	}

	return it, nil
}

// FindLogMetadataByTaskID queries the database to find the buildlogger logs
// that have given task id and optional tags, returning only the metadata for
// those logs.
func (dbc *DBConnector) FindLogMetadataByTaskID(ctx context.Context, taskID string, tags ...string) ([]model.APILog, error) {
	opts := dbModel.LogFindOptions{
		TimeRange: util.TimeRange{EndAt: time.Now()},
		Info: dbModel.LogInfo{
			TaskID: taskID,
			Tags:   tags,
		},
	}
	logs := dbModel.Logs{}
	logs.Setup(dbc.env)
	if err := logs.Find(ctx, opts); db.ResultsNotFound(err) {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("logs with task id '%s' not found", taskID),
		}
	} else if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("database error"),
		}
	}

	apiLogs := make([]model.APILog, len(logs.Logs))
	for i, log := range logs.Logs {
		if err := apiLogs[i].Import(log); err != nil {
			return nil, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    "corrupt data",
			}
		}
	}

	return apiLogs, nil
}

// FindLogsByTestName queries the database to find the buildlogger logs with
// the given task id, test name, and optional tags, returning the merged logs
// via a LogIterator with the corresponding time range.
func (dbc *DBConnector) FindLogsByTestName(ctx context.Context, taskID, testName string, tr util.TimeRange, tags ...string) (dbModel.LogIterator, error) {
	opts := dbModel.LogFindOptions{
		TimeRange: tr,
		Info: dbModel.LogInfo{
			TaskID: taskID,
			Tags:   tags,
		},
	}
	if testName != "" {
		opts.Info.TestName = testName
	} else {
		opts.Empty = dbModel.EmptyLogInfo{TestName: true}
	}
	logs := dbModel.Logs{}
	logs.Setup(dbc.env)
	if err := logs.Find(ctx, opts); db.ResultsNotFound(err) {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("logs with task id '%s' and test name '%s' not found", taskID, testName),
		}
	} else if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("database error"),
		}
	}

	logs.Setup(dbc.env)
	it, err := logs.Merge(ctx, false)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("%s", errors.Wrap(err, "problem downloading log")),
		}
	}

	return it, nil
}

// FindLogMetadataByTestName queries the database to find the buildlogger logs
// the given task id, test name, and optional tags, returning only the metadata
// for those logs.
func (dbc *DBConnector) FindLogMetadataByTestName(ctx context.Context, taskID, testName string, tags ...string) ([]model.APILog, error) {
	opts := dbModel.LogFindOptions{
		TimeRange: util.TimeRange{EndAt: time.Now()},
		Info: dbModel.LogInfo{
			TaskID: taskID,
			Tags:   tags,
		},
	}
	if testName != "" {
		opts.Info.TestName = testName
	} else {
		opts.Empty = dbModel.EmptyLogInfo{TestName: true}
	}
	logs := dbModel.Logs{}
	logs.Setup(dbc.env)
	if err := logs.Find(ctx, opts); db.ResultsNotFound(err) {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("logs with task id '%s' and test name '%s' not found", taskID, testName),
		}
	} else if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("database error"),
		}
	}

	apiLogs := make([]model.APILog, len(logs.Logs))
	for i, log := range logs.Logs {
		if err := apiLogs[i].Import(log); err != nil {
			return nil, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    "corrupt data",
			}
		}
	}

	return apiLogs, nil
}

///////////////////////////////
// MockConnector Implementation
///////////////////////////////

// FindLogByID queries the mock cache to find the buildlogger log with the
// given id returning a LogIterator with the corresponding time range.
func (mc *MockConnector) FindLogByID(ctx context.Context, id string, tr util.TimeRange) (dbModel.LogIterator, error) {
	log, ok := mc.CachedLogs[id]
	if !ok {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("log with id '%s' not found", id),
		}
	}

	opts := pail.LocalOptions{
		Path:   mc.Bucket,
		Prefix: log.Artifact.Prefix,
	}
	bucket, err := pail.NewLocalBucket(opts)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("%s", errors.Wrap(err, "problem creating bucket")),
		}
	}

	return dbModel.NewBatchedLogIterator(bucket, log.Artifact.Chunks, 2, tr, false), ctx.Err()
}

// FindLogMetadataByID queries the mock cache to find the buildlogger log with
// the given id returning its metadata only.
func (mc *MockConnector) FindLogMetadataByID(ctx context.Context, id string) (*model.APILog, error) {
	log, ok := mc.CachedLogs[id]
	if !ok {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("log with id '%s' not found", id),
		}
	}

	apiLog := &model.APILog{}
	if err := apiLog.Import(log); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    "corrupt data",
		}
	}

	return apiLog, ctx.Err()
}

// FindLogsByTaskID queries the mock cache to find the buildlogger logs with
// the given task id and optional tags, returning the merged logs via a
// LogIterator with the corresponding time range.
func (mc *MockConnector) FindLogsByTaskID(ctx context.Context, taskID string, tr util.TimeRange, tags ...string) (dbModel.LogIterator, error) {
	logs := []dbModel.Log{}
	for _, log := range mc.CachedLogs {
		if log.Info.TaskID == taskID {
			logs = append(logs, log)
		}
	}
	if len(logs) == 0 {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("logs with task id '%s' not found", taskID),
		}
	}

	sort.Slice(logs, func(i, j int) bool { return logs[i].CreatedAt.After(logs[j].CreatedAt) })

	its := []dbModel.LogIterator{}
	for _, log := range logs {
		if !containsTags(tags, log.Info.Tags) {
			continue
		}

		opts := pail.LocalOptions{
			Path:   mc.Bucket,
			Prefix: log.Artifact.Prefix,
		}
		bucket, err := pail.NewLocalBucket(opts)
		if err != nil {
			return nil, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    fmt.Sprintf("%s", errors.Wrap(err, "problem creating bucket")),
			}
		}

		its = append(its, dbModel.NewBatchedLogIterator(bucket, log.Artifact.Chunks, 2, tr, false))
	}

	return dbModel.NewMergingIterator(ctx, its...), ctx.Err()
}

// FindLogsByTaskID queries the mock cache to find the buildlogger logs that
// have the given task id and optional tags, returning only the metadata for
// those logs.
func (mc *MockConnector) FindLogMetadataByTaskID(ctx context.Context, taskID string, tags ...string) ([]model.APILog, error) {
	logs := []dbModel.Log{}
	for _, log := range mc.CachedLogs {
		if log.Info.TaskID == taskID {
			logs = append(logs, log)
		}
	}
	if len(logs) == 0 {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("logs with task id '%s' not found", taskID),
		}
	}

	sort.Slice(logs, func(i, j int) bool { return logs[i].CreatedAt.After(logs[j].CreatedAt) })

	apiLogs := []model.APILog{}
	for _, log := range logs {
		if !containsTags(tags, log.Info.Tags) {
			continue
		}

		apiLogs = append(apiLogs, model.APILog{})
		if err := apiLogs[len(apiLogs)-1].Import(log); err != nil {
			return nil, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    "corrupt data",
			}
		}
	}

	return apiLogs, ctx.Err()
}

// FindLogsByTestName queries the mock cache to find the buildlogger logs with
// the given task id, test name, and optional tags, returning the merged logs
// via a LogIterator with the corresponding time range.
func (mc *MockConnector) FindLogsByTestName(ctx context.Context, taskID, testName string, tr util.TimeRange, tags ...string) (dbModel.LogIterator, error) {
	logs := []dbModel.Log{}
	for _, log := range mc.CachedLogs {
		if log.Info.TaskID == taskID && log.Info.TestName == testName {
			logs = append(logs, log)
		}
	}
	if len(logs) == 0 {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("logs with task id '%s' and test name '%s' not found", taskID, testName),
		}
	}

	sort.Slice(logs, func(i, j int) bool { return logs[i].CreatedAt.After(logs[j].CreatedAt) })

	its := []dbModel.LogIterator{}
	for _, log := range logs {
		if !containsTags(tags, log.Info.Tags) {
			continue
		}

		opts := pail.LocalOptions{
			Path:   mc.Bucket,
			Prefix: log.Artifact.Prefix,
		}
		bucket, err := pail.NewLocalBucket(opts)
		if err != nil {
			return nil, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    fmt.Sprintf("%s", errors.Wrap(err, "problem creating bucket")),
			}
		}

		its = append(its, dbModel.NewBatchedLogIterator(bucket, log.Artifact.Chunks, 2, tr, false))
	}

	return dbModel.NewMergingIterator(ctx, its...), ctx.Err()
}

// FindLogMetadataByTestName queries the mock cache to find the buildlogger
// logs the given task id, test name, and optional tags, returning only the
// metadata for those logs.
func (mc *MockConnector) FindLogMetadataByTestName(ctx context.Context, taskID, testName string, tags ...string) ([]model.APILog, error) {
	logs := []dbModel.Log{}
	for _, log := range mc.CachedLogs {
		if log.Info.TaskID == taskID && log.Info.TestName == testName {
			logs = append(logs, log)
		}
	}
	if len(logs) == 0 {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("logs with task id '%s' and test name '%s' not found", taskID, testName),
		}
	}

	sort.Slice(logs, func(i, j int) bool { return logs[i].CreatedAt.After(logs[j].CreatedAt) })

	apiLogs := []model.APILog{}
	for _, log := range logs {
		if !containsTags(tags, log.Info.Tags) {
			continue
		}

		apiLogs = append(apiLogs, model.APILog{})
		if err := apiLogs[len(apiLogs)-1].Import(log); err != nil {
			return nil, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    "corrupt data",
			}
		}

	}

	return apiLogs, ctx.Err()
}
