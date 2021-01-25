package data

import (
	"context"
	"time"

	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/model"
)

// Connector abstracts the link between cedar's service and API layers,
// allowing for changes in the service architecture without forcing changes to
// the API.
type Connector interface {
	////////////////////
	// PerformanceResult
	////////////////////
	// FindPerformanceResultById returns the performance result with the
	// given id.
	FindPerformanceResultById(context.Context, string) (*model.APIPerformanceResult, error)
	// RemovePerformanceResultById removes the performance result with the
	// given id and its children.
	RemovePerformanceResultById(context.Context, string) (int, error)
	// FindPerformanceResultsByTaskId returns the performance results with
	// the given task id and that fall within given time range, filtered by
	// the optional tags.
	FindPerformanceResultsByTaskId(context.Context, string, dbModel.TimeRange, ...string) ([]model.APIPerformanceResult, error)
	// FindPerformanceResultsByTaskName returns the performance results
	// with the given task name and that fall within given time range,
	// filtered by the optional tags.
	FindPerformanceResultsByTaskName(context.Context, string, string, string, dbModel.TimeRange, int, ...string) ([]model.APIPerformanceResult, error)
	// FindPerformanceResultsByVersion returns the performance results with
	// the given version and that fall within given time range, filtered by
	// the optional tags.
	FindPerformanceResultsByVersion(context.Context, string, dbModel.TimeRange, ...string) ([]model.APIPerformanceResult, error)
	// FindPerformanceResultWithChildren returns the the performance result
	// with the given id and its children up to the given depth and
	// filtered by the optional tags.
	FindPerformanceResultWithChildren(context.Context, string, int, ...string) ([]model.APIPerformanceResult, error)
	// ScheduleSignalProcessingRecalculateJobs returns once a signal
	// processing recalculation job has been scheduled for each type of
	// test (project/variant/task/test combo).
	ScheduleSignalProcessingRecalculateJobs(context.Context) error

	//////////////////
	// Buildlogger Log
	//////////////////
	// FindLogById returns the buildlogger log with the given id. The time
	// returned is the next timestamp for pagination and the bool indicates
	// whether the log is paginated or not. If the log is not paginated,
	// the timestamp should be ignored.
	// ID, PrintTime, PrintPriority, TimeRange, and Limit are
	// respected from BuildloggerOptions.
	FindLogByID(context.Context, BuildloggerOptions) ([]byte, time.Time, bool, error)
	// FindLogMetadataByID returns the metadata for the buildlogger log
	// with the given id.
	FindLogMetadataByID(context.Context, string) (*model.APILog, error)
	// FindLogsByTaskID returns the buildlogger logs with the given task
	// id. The time returned is the next timestamp for pagination and the
	// bool indicates whether the logs are paginated or not. If the logs
	// are not paginated, the timestamp should be ignored.
	// TaskID, ProcessName, Execution, Tags, TimeRange, PrintTime,
	// PrintPriority, Limit, and Tail are respected from
	// BuildloggerOptions.
	FindLogsByTaskID(context.Context, BuildloggerOptions) ([]byte, time.Time, bool, error)
	// FindLogsByTaskID returns the metadata for the buildlogger logs with
	// the given task id and tags.
	FindLogMetadataByTaskID(context.Context, BuildloggerOptions) ([]model.APILog, error)
	// FindLogsByTestName returns the buildlogger logs with the given task
	// id and test name. The time returned is the next timestamp for
	// pagination and the bool indicates whether the logs are paginated
	// or not. If the logs are not paginated, the timestamp should be
	// ignored.
	// TaskID, TestName, ProcessName, Execution, Tags, TimeRange,
	// PrintTime, PrintPriority, Limit are respected from
	// BuildloggerOptions.
	FindLogsByTestName(context.Context, BuildloggerOptions) ([]byte, time.Time, bool, error)
	// FindLogsByTestName returns the metadata for the buildlogger logs
	// with the given task id, test name, and tags.
	FindLogMetadataByTestName(context.Context, BuildloggerOptions) ([]model.APILog, error)
	// FindGroupedLogs finds logs that are grouped via a "group id" held in
	// the tags field. These groups have a hierarchy of test level and task
	// level. This function returns logs with the given task id and group
	// id. The time returned is the next timestamp for pagination and the
	// bool indicates whether the logs are paginated or not. If the logs
	// are not paginated, the timestamp should be ignored.
	// TaskID, TestName, Tags, TimeRange, PrintTime, PrintPriority, and
	// Limit are respected from BuildloggerOptions.
	FindGroupedLogs(context.Context, BuildloggerOptions) ([]byte, time.Time, bool, error)

	///////////////
	// Test Results
	///////////////
	// FindTestResultsByTaskId queries the database to find all test
	// results with the given options.
	FindTestResultsByTaskId(context.Context, dbModel.TestResultsFindOptions) ([]model.APITestResult, error)
	// FindTestResultByTestName finds the test result of a single test,
	// specified by the given options.
	// If execution is not specified, this will return the test result from
	// the most recent.
	FindTestResultByTestName(context.Context, TestResultsTestNameOptions) (*model.APITestResult, error)

	///////////////////////
	// Historical Test Data
	///////////////////////
	// GetHistoricalTestData queries the historical test data using a
	// filter.
	GetHistoricalTestData(context.Context, dbModel.HistoricalTestDataFilter) ([]model.APIAggregatedHistoricalTestData, error)

	/////////////////
	// System Metrics
	/////////////////
	// FindSystemMetricsByType returns the raw data for the given task id,
	// execution, and metric type combination. It also returns the next
	// index to use for pagination.
	FindSystemMetricsByType(context.Context, dbModel.SystemMetricsFindOptions, dbModel.SystemMetricsDownloadOptions) ([]byte, int, error)
}

// BuildloggerOptions contains arguments for buildlogger related Connector
// functions.
type BuildloggerOptions struct {
	ID             string
	TaskID         string
	TestName       string
	Execution      int
	EmptyExecution bool
	ProcessName    string
	Group          string
	Tags           []string
	TimeRange      dbModel.TimeRange
	PrintTime      bool
	PrintPriority  bool
	Limit          int
	Tail           int
	SoftSizeLimit  int
}

// TestResultsOptions holds all values required to find a specific TestResults
// or TestResult object using connector functions.
type TestResultsOptions struct {
	TaskID         string
	TestName       string
	Execution      int
	EmptyExecution bool
}

// TestResultsTestNameOptions holds all values required to find a specific TestResults
// or TestResult object using connector functions.
type TestResultsTestNameOptions struct {
	TaskID         string
	TestName       string
	Execution      int
	EmptyExecution bool
}
