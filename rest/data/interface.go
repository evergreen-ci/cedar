package data

import (
	"context"

	"github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/cedar/util"
)

type Connector interface {
	// PerformanceResult
	FindPerformanceResultById(string) (*model.APIPerformanceResult, error)
	RemovePerformanceResultById(string) (int, error)
	FindPerformanceResultsByTaskId(string, util.TimeRange, ...string) ([]model.APIPerformanceResult, error)
	FindPerformanceResultsByTaskName(string, string, string, util.TimeRange, int, ...string) ([]model.APIPerformanceResult, error)
	FindPerformanceResultsByVersion(string, util.TimeRange, ...string) ([]model.APIPerformanceResult, error)
	FindPerformanceResultWithChildren(string, int, ...string) ([]model.APIPerformanceResult, error)

	// Log
	FindLogById(context.Context, string) (*model.APILog, error)
}
