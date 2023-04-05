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
	// GetBaseURL returns the API service URL.
	GetBaseURL() string

	////////////////////
	// PerformanceResult
	////////////////////
	// FindPerformanceResultById returns the performance result with the
	// given id.
	FindPerformanceResultById(context.Context, string) (*model.APIPerformanceResult, error)
	// RemovePerformanceResultById removes the performance result with the
	// given ID and its children.
	RemovePerformanceResultById(context.Context, string) (int, error)
	// FindPerformanceResults returns all performance results that match the
	// given options.
	FindPerformanceResults(context.Context, PerformanceOptions) ([]model.APIPerformanceResult, error)
	// FindPerformanceResultWithChildren returns the the performance result
	// with the given ID and its children up to the given depth and
	// filtered by the optional tags.
	FindPerformanceResultWithChildren(context.Context, string, int, ...string) ([]model.APIPerformanceResult, error)
	// ScheduleSignalProcessingRecalculateJobs returns once a signal
	// processing recalculation job has been scheduled for each type of
	// test (project/variant/task/test combo).
	ScheduleSignalProcessingRecalculateJobs(context.Context) error

	//////////////////
	// Buildlogger Log
	//////////////////
	// FindLogById returns the buildlogger log with the given ID. The time
	// returned is the next timestamp for pagination and the bool indicates
	// whether the log is paginated or not. If the log is not paginated,
	// the timestamp should be ignored.
	// ID, PrintTime, PrintPriority, TimeRange, Limit, and SoftSizeLimit
	// are respected from BuildloggerOptions.
	FindLogByID(context.Context, BuildloggerOptions) ([]byte, time.Time, bool, error)
	// FindLogMetadataByID returns the metadata for the buildlogger log
	// with the given ID.
	FindLogMetadataByID(context.Context, string) (*model.APILog, error)
	// FindLogsByTaskID returns the buildlogger logs with the given task
	// id. The time returned is the next timestamp for pagination and the
	// bool indicates whether the logs are paginated or not. If the logs
	// are not paginated, the timestamp should be ignored.
	// TaskID, ProcessName, Execution, Tags, TimeRange, PrintTime,
	// PrintPriority, Limit, Tail, and SoftSizeLimit are respected from
	// BuildloggerOptions.
	FindLogsByTaskID(context.Context, BuildloggerOptions) ([]byte, time.Time, bool, error)
	// FindLogsByTaskID returns the metadata for the buildlogger logs with
	// the given task ID and tags.
	FindLogMetadataByTaskID(context.Context, BuildloggerOptions) ([]model.APILog, error)
	// FindLogsByTestName returns the buildlogger logs with the given task
	// ID and test name. The time returned is the next timestamp for
	// pagination and the bool indicates whether the logs are paginated
	// or not. If the logs are not paginated, the timestamp should be
	// ignored.
	// TaskID, TestName, ProcessName, Execution, Tags, TimeRange,
	// PrintTime, PrintPriority, Limit, and SoftSizeLimit are respected
	// from BuildloggerOptions.
	FindLogsByTestName(context.Context, BuildloggerOptions) ([]byte, time.Time, bool, error)
	// FindLogsByTestName returns the metadata for the buildlogger logs
	// with the given task ID, test name, and tags.
	FindLogMetadataByTestName(context.Context, BuildloggerOptions) ([]model.APILog, error)
	// FindGroupedLogs finds logs that are grouped via a "group id" held in
	// the tags field. These groups have a hierarchy of test level and task
	// level. This function returns logs with the given task ID and group
	// id. The time returned is the next timestamp for pagination and the
	// bool indicates whether the logs are paginated or not. If the logs
	// are not paginated, the timestamp should be ignored.
	// TaskID, TestName, Execution, Tags, TimeRange, PrintTime,
	// PrintPriority, Limit, and SoftSizeLimit are respected from
	// BuildloggerOptions.
	FindGroupedLogs(context.Context, BuildloggerOptions) ([]byte, time.Time, bool, error)

	///////////////
	// Test Results
	///////////////
	// FindTestResults returns the merged test results of the given tasks
	// and optional filter, sort, and pagination options.
	FindTestResults(context.Context, []TestResultsTaskOptions, *TestResultsFilterAndSortOptions) (*model.APITestResults, error)
	// FindTestResultsStats returns basic aggregated stats of test results
	// results for the given tasks.
	FindTestResultsStats(context.Context, []TestResultsTaskOptions) (*model.APITestResultsStats, error)
	// FindTestResultsSample returns a merged list of the failed test
	// results samples for the given tasks.
	FindFailedTestResultsSample(context.Context, []TestResultsTaskOptions) ([]string, error)
	// FindFailedTestResultsSamples returns failed test result samples for
	// the given tasks and optional regex filters.
	FindFailedTestResultsSamples(context.Context, []TestResultsTaskOptions, []string) ([]model.APITestResultsSample, error)

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
	EmptyTestName  bool
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

// TestResultsTaskOptions specify the arguments for fetching test results by
// task using the Connector functions.
type TestResultsTaskOptions struct {
	TaskID    string `json:"task_id"`
	Execution int    `json:"execution"`
}

// TestResultsSortBy describes the property by which to sort a set of test
// results using the Connector functions.
type TestResultsSortBy struct {
	Key          string `json:"key"`
	SortOrderDSC bool   `json:"sort_order_dsc"`
}

// TestResultsFilterAndSortOptions holds all values required for filtering,
// sorting, and paginating test results using the Connector functions.
type TestResultsFilterAndSortOptions struct {
	TestName  string                   `json:"test_name"`
	Statuses  []string                 `json:"statuses"`
	GroupID   string                   `json:"group_id"`
	Sort      []TestResultsSortBy      `json:"sort"`
	Limit     int                      `json:"limit"`
	Page      int                      `json:"page"`
	BaseTasks []TestResultsTaskOptions `json:"base_tasks"`

	// TODO (EVG-14306): Remove these two fields once Evergreen's GraphQL
	// service is no longer using them.
	SortBy       string `json:"sort_by"`
	SortOrderDSC bool   `json:"sort_order_dsc"`
}

// PerformanceOptions holds all values required to find a specific
// PerformanceResult or PerformanceResults using connector functions.
type PerformanceOptions struct {
	Project   string
	Version   string
	Variant   string
	TaskID    string
	Execution int
	TaskName  string
	Tags      []string
	Interval  dbModel.TimeRange
	Limit     int
	Skip      int
}
