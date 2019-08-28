package data

import (
	"context"

	dbModel "github.com/evergreen-ci/cedar/model"
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
	FindPerformanceResultById(string) (*model.APIPerformanceResult, error)
	// RemovePerformanceResultById removes the performance result with the
	// given id and its children.
	RemovePerformanceResultById(string) (int, error)
	// FindPerformanceResultsByTaskId returns the performance results with
	// the given task id and that fall within given time range, filtered by
	// the optional tags.
	FindPerformanceResultsByTaskId(string, util.TimeRange, ...string) ([]model.APIPerformanceResult, error)
	// FindPerformanceResultsByTaskName returns the performance results
	// with the given task name and that fall within given time range,
	// filtered by the optional tags.
	FindPerformanceResultsByTaskName(string, string, string, util.TimeRange, int, ...string) ([]model.APIPerformanceResult, error)
	// FindPerformanceResultsByVersion returns the performance results with
	// the given version and that fall within given time range, filtered by
	// the optional tags.
	FindPerformanceResultsByVersion(string, util.TimeRange, ...string) ([]model.APIPerformanceResult, error)
	// FindPerformanceResultWithChildren returns the the performance result
	// with the given id and its children up to the given depth and
	// filtered by the optional tags.
	FindPerformanceResultWithChildren(string, int, ...string) ([]model.APIPerformanceResult, error)

	//////////////////
	// Buildlogger Log
	//////////////////
	// FindLogById returns the buildlogger log with the given id and a
	// LogIterator with the corresponding time range.
	FindLogById(context.Context, string, util.TimeRange) (*model.APILog, dbModel.LogIterator, error)
}
