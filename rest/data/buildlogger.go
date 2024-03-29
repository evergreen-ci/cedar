package data

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"time"

	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/gimlet"
	"github.com/evergreen-ci/pail"
	"github.com/mongodb/anser/db"
	"github.com/pkg/errors"
)

/////////////////////////////
// DBConnector Implementation
/////////////////////////////

func (dbc *DBConnector) FindLogByID(ctx context.Context, opts BuildloggerOptions) ([]byte, time.Time, bool, error) {
	var (
		data      []byte
		paginated bool
	)

	log := dbModel.Log{ID: opts.ID}
	log.Setup(dbc.env)
	if err := log.Find(ctx); db.ResultsNotFound(err) {
		return data, time.Time{}, paginated, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("log '%s' not found", opts.ID),
		}
	} else if err != nil {
		return data, time.Time{}, paginated, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "finding log '%s'", opts.ID).Error(),
		}
	}

	log.Setup(dbc.env)
	it, err := log.Download(ctx, opts.TimeRange)
	if err != nil {
		return data, time.Time{}, paginated, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "downloading log '%s'", opts.ID).Error(),
		}
	}

	opts.Tail = 0
	data, paginated, err = paginateData(ctx, it, opts)
	if err != nil {
		return data, time.Time{}, paginated, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "paginating log '%s'", opts.ID).Error(),
		}
	}

	next := it.Item().Timestamp
	if it.Exhausted() {
		next = time.Time{}
	}
	return data, next, paginated, nil
}

func (dbc *DBConnector) FindLogMetadataByID(ctx context.Context, id string) (*model.APILog, error) {
	log := dbModel.Log{ID: id}
	log.Setup(dbc.env)
	if err := log.Find(ctx); db.ResultsNotFound(err) {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("log '%s' not found", id),
		}
	} else if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "finding log '%s'", id).Error(),
		}
	}

	apiLog := &model.APILog{}
	if err := apiLog.Import(log); err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "corrupt data for log '%s'", id).Error(),
		}
	}

	return apiLog, nil
}

func (dbc *DBConnector) FindLogsByTaskID(ctx context.Context, opts BuildloggerOptions) ([]byte, time.Time, bool, error) {
	var (
		data      []byte
		paginated bool
	)

	dbOpts := dbModel.LogFindOptions{
		TimeRange: opts.TimeRange,
		Info: dbModel.LogInfo{
			TaskID:      opts.TaskID,
			Execution:   opts.Execution,
			ProcessName: opts.ProcessName,
			Tags:        opts.Tags,
		},
		LatestExecution: opts.EmptyExecution,
	}
	logs := dbModel.Logs{}
	logs.Setup(dbc.env)
	if err := logs.Find(ctx, dbOpts); db.ResultsNotFound(err) {
		return data, time.Time{}, paginated, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("logs with task ID '%s' not found", opts.TaskID),
		}
	} else if err != nil {
		return data, time.Time{}, paginated, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "finding logs with task ID '%s'", opts.TaskID).Error(),
		}
	}

	logs.Setup(dbc.env)
	it, err := logs.Merge(ctx)
	if err != nil {
		return data, time.Time{}, paginated, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "downloading logs with task ID '%s'", opts.TaskID).Error(),
		}
	}

	data, paginated, err = paginateData(ctx, it, opts)
	if err != nil {
		return data, time.Time{}, paginated, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "paginating logs with task ID '%s'", opts.TaskID).Error(),
		}
	}

	next := it.Item().Timestamp
	if it.Exhausted() {
		next = time.Time{}
	}
	return data, next, paginated, nil
}

func (dbc *DBConnector) FindLogMetadataByTaskID(ctx context.Context, opts BuildloggerOptions) ([]model.APILog, error) {
	dbOpts := dbModel.LogFindOptions{
		TimeRange: dbModel.TimeRange{EndAt: time.Now()},
		Info: dbModel.LogInfo{
			TaskID:    opts.TaskID,
			Execution: opts.Execution,
			Tags:      opts.Tags,
		},
		LatestExecution: opts.EmptyExecution,
	}
	logs := dbModel.Logs{}
	logs.Setup(dbc.env)
	if err := logs.Find(ctx, dbOpts); db.ResultsNotFound(err) {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("logs with task ID '%s' not found", opts.TaskID),
		}
	} else if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "finding logs with task ID '%s'", opts.TaskID).Error(),
		}
	}

	apiLogs := make([]model.APILog, len(logs.Logs))
	for i, log := range logs.Logs {
		if err := apiLogs[i].Import(log); err != nil {
			return nil, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    errors.Wrapf(err, "corrupt data for logs with task ID '%s'", opts.TaskID).Error(),
			}
		}
	}

	return apiLogs, nil
}

func (dbc *DBConnector) FindLogsByTestName(ctx context.Context, opts BuildloggerOptions) ([]byte, time.Time, bool, error) {
	var (
		data      []byte
		paginated bool
	)

	opts.Group = ""
	it, err := dbc.findLogsByTestName(ctx, &opts)
	if err != nil {
		return data, time.Time{}, paginated, err
	}

	opts.Tail = 0
	data, paginated, err = paginateData(ctx, it, opts)
	if err != nil {
		return data, time.Time{}, paginated, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "paginating logs with task ID '%s' and test name '%s'", opts.TaskID, opts.TestName).Error(),
		}
	}

	next := it.Item().Timestamp
	if it.Exhausted() {
		next = time.Time{}
	}
	return data, next, paginated, nil
}

func (dbc *DBConnector) FindLogMetadataByTestName(ctx context.Context, opts BuildloggerOptions) ([]model.APILog, error) {
	dbOpts := dbModel.LogFindOptions{
		TimeRange: dbModel.TimeRange{EndAt: time.Now()},
		Info: dbModel.LogInfo{
			TaskID:    opts.TaskID,
			Execution: opts.Execution,
			Tags:      opts.Tags,
		},
		LatestExecution: opts.EmptyExecution,
	}
	if opts.TestName != "" {
		dbOpts.Info.TestName = opts.TestName
	} else {
		dbOpts.EmptyTestName = true
	}
	logs := dbModel.Logs{}
	logs.Setup(dbc.env)
	if err := logs.Find(ctx, dbOpts); db.ResultsNotFound(err) {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("logs with task ID '%s' and test name '%s' not found", opts.TaskID, opts.TestName),
		}
	} else if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "finding logs with task ID '%s' and test name '%s'", opts.TaskID, opts.TestName).Error(),
		}
	}

	apiLogs := make([]model.APILog, len(logs.Logs))
	for i, log := range logs.Logs {
		if err := apiLogs[i].Import(log); err != nil {
			return nil, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    errors.Wrapf(err, "corrupt data for logs with task ID '%s' and test name '%s'", opts.TaskID, opts.TestName).Error(),
			}
		}
	}

	return apiLogs, nil
}

func (dbc *DBConnector) FindGroupedLogs(ctx context.Context, opts BuildloggerOptions) ([]byte, time.Time, bool, error) {
	var (
		data      []byte
		paginated bool
	)

	opts.ProcessName = ""
	its := []dbModel.LogIterator{}
	if opts.TestName != "" {
		it, err := dbc.findLogsByTestName(ctx, &opts)
		if err != nil {
			return data, time.Time{}, paginated, err
		}
		its = append(its, it)
		opts.EmptyTestName = true
		// Need to set this to false since the last call to findLogsByTestName
		// has found the latest execution.
		opts.EmptyExecution = false
	}
	it, err := dbc.findLogsByTestName(ctx, &opts)
	if err == nil {
		its = append(its, it)
	} else if errResp, ok := err.(gimlet.ErrorResponse); !ok || opts.TestName == "" || errResp.StatusCode != http.StatusNotFound {
		return data, time.Time{}, paginated, err
	}
	it = dbModel.NewMergingIterator(its...)

	opts.Tail = 0
	data, paginated, err = paginateData(ctx, it, opts)
	if err != nil {
		return data, time.Time{}, paginated, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "paginating grouped logs with task ID '%s' and test name '%s'", opts.TaskID, opts.TestName).Error(),
		}
	}

	next := it.Item().Timestamp
	if it.Exhausted() {
		next = time.Time{}
	}
	return data, next, paginated, nil
}

func (dbc *DBConnector) findLogsByTestName(ctx context.Context, opts *BuildloggerOptions) (dbModel.LogIterator, error) {
	dbOpts := dbModel.LogFindOptions{
		TimeRange: opts.TimeRange,
		Info: dbModel.LogInfo{
			TaskID:      opts.TaskID,
			TestName:    opts.TestName,
			ProcessName: opts.ProcessName,
			Execution:   opts.Execution,
			Tags:        opts.Tags,
		},
		EmptyTestName:   opts.EmptyTestName,
		LatestExecution: opts.EmptyExecution,
		Group:           opts.Group,
	}
	logs := dbModel.Logs{}
	logs.Setup(dbc.env)
	if err := logs.Find(ctx, dbOpts); db.ResultsNotFound(err) {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("logs with task ID '%s' and test name '%s' not found", opts.TaskID, opts.TestName),
		}
	} else if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "finding logs with task ID '%s' and test name '%s'", opts.TaskID, opts.TestName).Error(),
		}
	}
	opts.Execution = logs.Logs[0].Info.Execution

	logs.Setup(dbc.env)
	it, err := logs.Merge(ctx)
	if err != nil {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "downloading logs with task ID '%s' and test name '%s'", opts.TaskID, opts.TestName).Error(),
		}
	}

	return it, nil
}

///////////////////////////////
// MockConnector Implementation
///////////////////////////////

func (mc *MockConnector) FindLogByID(ctx context.Context, opts BuildloggerOptions) ([]byte, time.Time, bool, error) {
	var (
		data      []byte
		paginated bool
	)

	log, ok := mc.CachedLogs[opts.ID]
	if !ok {
		return data, time.Time{}, paginated, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("log '%s' not found", opts.ID),
		}
	}

	bucketOpts := pail.LocalOptions{
		Path:   mc.Bucket,
		Prefix: log.Artifact.Prefix,
	}
	bucket, err := pail.NewLocalBucket(bucketOpts)
	if err != nil {
		return data, time.Time{}, paginated, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "creating bucket").Error(),
		}
	}
	it := dbModel.NewBatchedLogIterator(bucket, log.Artifact.Chunks, 2, opts.TimeRange)

	opts.Tail = 0
	data, paginated, err = paginateData(ctx, it, opts)
	if err != nil {
		return data, time.Time{}, paginated, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "paginating log").Error(),
		}
	}

	next := it.Item().Timestamp
	if it.Exhausted() {
		next = time.Time{}
	}
	return data, next, paginated, ctx.Err()
}

func (mc *MockConnector) FindLogMetadataByID(ctx context.Context, id string) (*model.APILog, error) {
	log, ok := mc.CachedLogs[id]
	if !ok {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("log '%s' not found", id),
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

func (mc *MockConnector) FindLogsByTaskID(ctx context.Context, opts BuildloggerOptions) ([]byte, time.Time, bool, error) {
	var (
		data      []byte
		paginated bool
	)
	logs := []dbModel.Log{}
	for _, log := range mc.CachedLogs {
		if log.Info.TaskID == opts.TaskID {
			logs = append(logs, log)
		}
	}
	if len(logs) == 0 {
		return data, time.Time{}, paginated, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("logs with task ID '%s' not found", opts.TaskID),
		}
	}

	if opts.EmptyExecution {
		opts.Execution = getMaxExecution(logs)
	}
	sort.Slice(logs, func(i, j int) bool { return logs[i].CreatedAt.After(logs[j].CreatedAt) })

	its := []dbModel.LogIterator{}
	for _, log := range logs {
		if opts.ProcessName != "" && opts.ProcessName != log.Info.ProcessName {
			continue
		}
		if opts.Execution != log.Info.Execution {
			continue
		}
		if !containsTags(opts.Tags, log.Info.Tags) {
			continue
		}

		bucketOpts := pail.LocalOptions{
			Path:   mc.Bucket,
			Prefix: log.Artifact.Prefix,
		}
		bucket, err := pail.NewLocalBucket(bucketOpts)
		if err != nil {
			return data, time.Time{}, paginated, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    errors.Wrap(err, "creating bucket").Error(),
			}
		}

		its = append(its, dbModel.NewBatchedLogIterator(bucket, log.Artifact.Chunks, 2, opts.TimeRange))
	}
	it := dbModel.NewMergingIterator(its...)

	var err error
	data, paginated, err = paginateData(ctx, it, opts)
	if err != nil {
		return data, time.Time{}, paginated, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "paginating log").Error(),
		}
	}

	next := it.Item().Timestamp
	if it.Exhausted() {
		next = time.Time{}
	}
	return data, next, paginated, ctx.Err()
}

func (mc *MockConnector) FindLogMetadataByTaskID(ctx context.Context, opts BuildloggerOptions) ([]model.APILog, error) {
	logs := []dbModel.Log{}
	for _, log := range mc.CachedLogs {
		if log.Info.TaskID == opts.TaskID {
			logs = append(logs, log)
		}
	}
	if len(logs) == 0 {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("logs with task ID '%s' not found", opts.TaskID),
		}
	}

	if opts.EmptyExecution {
		opts.Execution = getMaxExecution(logs)
	}
	sort.Slice(logs, func(i, j int) bool { return logs[i].CreatedAt.After(logs[j].CreatedAt) })

	apiLogs := []model.APILog{}
	for _, log := range logs {
		if opts.Execution != log.Info.Execution {
			continue
		}
		if !containsTags(opts.Tags, log.Info.Tags) {
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

func (mc *MockConnector) FindLogsByTestName(ctx context.Context, opts BuildloggerOptions) ([]byte, time.Time, bool, error) {
	var (
		data      []byte
		paginated bool
	)

	opts.Group = ""
	it, err := mc.findLogsByTestName(ctx, &opts)
	if err != nil {
		return data, time.Time{}, paginated, err
	}

	opts.Tail = 0
	data, paginated, err = paginateData(ctx, it, opts)
	if err != nil {
		return data, time.Time{}, paginated, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "paginating log").Error(),
		}
	}

	next := it.Item().Timestamp
	if it.Exhausted() {
		next = time.Time{}
	}
	return data, next, paginated, ctx.Err()
}

func (mc *MockConnector) FindLogMetadataByTestName(ctx context.Context, opts BuildloggerOptions) ([]model.APILog, error) {
	logs := []dbModel.Log{}
	for _, log := range mc.CachedLogs {
		if opts.EmptyTestName && log.Info.TestName != "" {
			continue
		}
		if !opts.EmptyTestName && opts.TestName != "" && log.Info.TestName != opts.TestName {
			continue
		}
		if log.Info.TaskID == opts.TaskID {
			logs = append(logs, log)
		}
	}

	if len(logs) == 0 {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("logs with task ID '%s' and test name '%s' not found", opts.TaskID, opts.TestName),
		}
	}

	if opts.EmptyExecution {
		opts.Execution = getMaxExecution(logs)
	}
	sort.Slice(logs, func(i, j int) bool { return logs[i].CreatedAt.After(logs[j].CreatedAt) })

	apiLogs := []model.APILog{}
	for _, log := range logs {
		if opts.Execution != log.Info.Execution {
			continue
		}
		if !containsTags(opts.Tags, log.Info.Tags) {
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

func (mc *MockConnector) FindGroupedLogs(ctx context.Context, opts BuildloggerOptions) ([]byte, time.Time, bool, error) {
	var (
		data      []byte
		paginated bool
	)

	opts.ProcessName = ""
	its := []dbModel.LogIterator{}
	if opts.TestName != "" {
		it, err := mc.findLogsByTestName(ctx, &opts)
		if err != nil {
			return data, time.Time{}, paginated, err
		}
		its = append(its, it)
		opts.EmptyTestName = true
		// Need to set this to false since the last call to findLogsByTestName
		// has found the latest execution.
		opts.EmptyExecution = false
	}
	it, err := mc.findLogsByTestName(ctx, &opts)
	if err == nil {
		its = append(its, it)
	} else if errResp, ok := err.(gimlet.ErrorResponse); !ok || opts.TestName == "" || errResp.StatusCode != http.StatusNotFound {
		return data, time.Time{}, paginated, err
	}
	it = dbModel.NewMergingIterator(its...)

	opts.Tail = 0
	data, paginated, err = paginateData(ctx, it, opts)
	if err != nil {
		return data, time.Time{}, paginated, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrap(err, "paginating log").Error(),
		}
	}

	next := it.Item().Timestamp
	if it.Exhausted() {
		next = time.Time{}
	}
	return data, next, paginated, ctx.Err()
}

func (mc *MockConnector) findLogsByTestName(ctx context.Context, opts *BuildloggerOptions) (dbModel.LogIterator, error) {
	logs := []dbModel.Log{}
	for _, log := range mc.CachedLogs {
		if log.Info.TaskID == opts.TaskID && containsTags(opts.Tags, log.Info.Tags) {
			if opts.Group != "" && !containsTags([]string{opts.Group}, log.Info.Tags) {
				continue
			}
			if opts.EmptyTestName && log.Info.TestName != "" {
				continue
			}
			if !opts.EmptyTestName && opts.TestName != "" && log.Info.TestName != opts.TestName {
				continue
			}
			if opts.ProcessName != "" && log.Info.ProcessName != opts.ProcessName {
				continue
			}
			logs = append(logs, log)
		}
	}

	if len(logs) == 0 {
		return nil, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("logs with task ID '%s' and test name '%s' not found", opts.TaskID, opts.TestName),
		}
	}

	if opts.EmptyExecution {
		opts.Execution = getMaxExecution(logs)
	}
	sort.Slice(logs, func(i, j int) bool { return logs[i].CreatedAt.After(logs[j].CreatedAt) })

	its := []dbModel.LogIterator{}
	for _, log := range logs {
		if opts.Execution != log.Info.Execution {
			continue
		}

		bucketOpts := pail.LocalOptions{
			Path:   mc.Bucket,
			Prefix: log.Artifact.Prefix,
		}
		bucket, err := pail.NewLocalBucket(bucketOpts)
		if err != nil {
			return nil, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    errors.Wrap(err, "creating bucket").Error(),
			}
		}

		its = append(its, dbModel.NewBatchedLogIterator(bucket, log.Artifact.Chunks, 2, opts.TimeRange))
	}

	return dbModel.NewMergingIterator(its...), ctx.Err()
}

func getMaxExecution(logs []dbModel.Log) int {
	max := 0
	for _, log := range logs {
		if log.Info.Execution > max {
			max = log.Info.Execution
		}
	}

	return max
}

func paginateData(ctx context.Context, it dbModel.LogIterator, opts BuildloggerOptions) ([]byte, bool, error) {
	readerOpts := dbModel.LogIteratorReaderOptions{
		Limit:         opts.Limit,
		TailN:         opts.Tail,
		PrintTime:     opts.PrintTime,
		PrintPriority: opts.PrintPriority,
	}

	var paginated bool
	if opts.SoftSizeLimit > 0 && readerOpts.Limit <= 0 && readerOpts.TailN <= 0 {
		readerOpts.SoftSizeLimit = opts.SoftSizeLimit
		paginated = true
	}
	reader := dbModel.NewLogIteratorReader(ctx, it, readerOpts)

	data, err := ioutil.ReadAll(reader)
	return data, paginated, err
}
