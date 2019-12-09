package data

import (
	"context"
	"io"

	"github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/cedar/util"
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
	FindPerformanceResultsByTaskId(context.Context, string, util.TimeRange, ...string) ([]model.APIPerformanceResult, error)
	// FindPerformanceResultsByTaskName returns the performance results
	// with the given task name and that fall within given time range,
	// filtered by the optional tags.
	FindPerformanceResultsByTaskName(context.Context, string, string, string, util.TimeRange, int, ...string) ([]model.APIPerformanceResult, error)
	// FindPerformanceResultsByVersion returns the performance results with
	// the given version and that fall within given time range, filtered by
	// the optional tags.
	FindPerformanceResultsByVersion(context.Context, string, util.TimeRange, ...string) ([]model.APIPerformanceResult, error)
	// FindPerformanceResultWithChildren returns the the performance result
	// with the given id and its children up to the given depth and
	// filtered by the optional tags.
	FindPerformanceResultWithChildren(context.Context, string, int, ...string) ([]model.APIPerformanceResult, error)
	// ScheduleSignalProcessingRecalculateJobs returns once a signal processing recalculation
	// job has been scheduled for each type of test (project/variant/task/test combo)
	ScheduleSignalProcessingRecalculateJobs(context.Context) error

	//////////////////
	// Buildlogger Log
	//////////////////
	// FindLogByID returns the buildlogger log with the given id as a
	// LogIterator with the corresponding time range.
	FindLogByID(context.Context, string, util.TimeRange) (io.Reader, error)
	// FindLogMetadataByID returns the metadata for the buildlogger log
	// with the given id.
	FindLogMetadataByID(context.Context, string) (*model.APILog, error)
	// FindLogsByTaskID returns the buildlogger logs with the given task id
	// and optional tags merged via a LogIterator with the corresponding
	// time range.
	FindLogsByTaskID(context.Context, string, util.TimeRange, int, ...string) (io.Reader, error)
	// FindLogsByTaskID returns the metadata for the buildlogger logs with
	// the given task id and optional tags.
	FindLogMetadataByTaskID(context.Context, string, ...string) ([]model.APILog, error)
	// FindLogsByTestName returns the buildlogger logs with the given task
	// id, test name, and optional tags.
	FindLogsByTestName(context.Context, string, string, util.TimeRange, ...string) (io.Reader, error)
	// FindLogsByTestName returns the metadata for the buildlogger logs
	// with the given task id, test name, and optional tags.
	FindLogMetadataByTestName(context.Context, string, string, ...string) ([]model.APILog, error)
	// FindGroupedLogs finds logs that are grouped via a "group id" held in
	// the tags field. These groups have a hierarchy of test level and task
	// level. This function returns logs with the given task id, test name
	// (or empty test name), group id, time range, and optional tags.
	FindGroupedLogs(context.Context, string, string, string, util.TimeRange, ...string) (io.Reader, error)
}
