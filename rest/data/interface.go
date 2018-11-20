package data

import (
	"fmt"

	"github.com/evergreen-ci/sink"
)

type Connector interface {
	// Get and Set Env.
	GetEnv() sink.Environment
	SetEnv(sink.Environment)

	// TODO: Perf functions.
	FindPerformanceResultById(DBPerformanceResultInput) (*PerformanceResult, error)
	FindPerformancesByTaskId(DBPerformanceResultInput) ([]*PerformanceResult, error)
	FindPerformanceResultsByVersion(DBPerformanceResultInput) ([]*PerformanceResult, error)
	FindPerformanceResultChildren(DBPerformanceResultInput) ([]*PerformanceResult, error)
}
