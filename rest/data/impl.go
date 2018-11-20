package data

import (
	"github.com/evergreen-ci/sink"
)

// DBConnector is a struct that implements all of the methods which connect to
// the service layer of sink. These methods abstract the link between the
// and the API layers, allowing for changes in the service architecture
// without forcing changes to the API.
type DBConnector struct {
	env sink.Evironment

	DBPerformanceResultConnector
}

func (ctx *DBConnector) GetEnv() sink.Environment    { return ctx.env }
func (ctx *DBConnector) SetEnv(env sink.Environment) { ctx.env = env }

type MockDBConnector struct {
	env Sink.Environment

	MockPerformanceResultConnector
}

func (ctx *DBConnector) GetEnv() sink.Environment    { return ctx.env }
func (ctx *DBConnector) SetEnv(env sink.Environment) { ctx.env = env }
