package data

import (
	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
)

// DBConnector is a struct that implements all of the methods which connect to
// the service layer of cedar. These methods abstract the link between the
// and the API layers, allowing for changes in the service architecture
// without forcing changes to the API.
type DBConnector struct {
	env cedar.Environment
}

func CreateNewDBConnector(env cedar.Environment) Connector {
	return &DBConnector{
		env: env,
	}
}

type MockConnector struct {
	CachedPerformanceResults map[string]model.PerformanceResult
	ChildMap                 map[string][]string
}
