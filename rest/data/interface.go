package data

import (
	"github.com/evergreen-ci/sink/rest/model"
	"github.com/evergreen-ci/sink/util"
)

type Connector interface {
	// PerformanceResult
	FindPerformanceResultById(string) (*model.APIPerformanceResult, error)
	FindPerformanceResultsByTaskId(string, util.TimeRange, ...string) ([]model.APIPerformanceResult, error)
	FindPerformanceResultsByVersion(string, util.TimeRange, ...string) ([]model.APIPerformanceResult, error)
	FindPerformanceResultWithChildren(string, int, ...string) ([]model.APIPerformanceResult, error)
}
