package data

import (
	"github.com/evergreen-ci/sink"
)

// DBConnector is a struct that implements all of the methods which connect to
// the service layer of sink. These methods abstract the link between the
// and the API layers, allowing for changes in the service architecture
// without forcing changes to the API.
type DBConnector struct {
	env sink.Environment
}

func CreateDBConnector(env sink.Environment) Connector {
	return &DBConnector{
		env: env,
	}
}
