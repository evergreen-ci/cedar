package data

import (
	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
)

// DBConnector is a struct that implements the Connector interface backed by
// the service layer of cedar.
type DBConnector struct {
	env cedar.Environment
}

// CreateNewDBConnector is the entry point for creating a new Connector backed
// by DBConnector.
func CreateNewDBConnector(env cedar.Environment) Connector {
	return &DBConnector{
		env: env,
	}
}

// MockConnector is a struct that implements the Connector interface backed by
// the a mock cedar service layer.
type MockConnector struct {
	CachedPerformanceResults map[string]model.PerformanceResult
	ChildMap                 map[string][]string
	CachedLogs               map[string]model.Log

	env cedar.Environment
}
