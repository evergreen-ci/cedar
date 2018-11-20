package data

import (
	"github.com/evergreen-ci/sink/model"
	"github.com/evergreen-ci/sink/util"
)

type Connector interface {
	// PerformanceResult
	FindPerformanceResultById(string) (*model.PerformanceResult, error)
	FindPerformanceResultsByTaskId(string, util.TimeRange, ...string) ([]model.PerformanceResult, error)
	FindPerformanceResultsByVersion(string, util.TimeRange, ...string) ([]model.PerformanceResult, error)
	FindPerformanceResultWithChildren(string, util.TimeRange, int, ...string) ([]model.PerformanceResult, error)
}
