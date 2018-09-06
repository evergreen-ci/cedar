package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/evergreen-ci/sink"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type commonModel interface {
	Setup(e sink.Environment)
	IsNil() bool
	Find() error
	Save() error
}

type commonModelFactory func() commonModel

func TestModelInterface(t *testing.T) {
	models := []commonModelFactory{
		func() commonModel { return &SinkConfig{} },
		func() commonModel { return &CostConfig{} },
		func() commonModel { return &CostReport{} },
		func() commonModel { return &CostReportSummary{} },
		func() commonModel { return &GraphMetadata{} },
		func() commonModel { return &GraphEdge{} },
		func() commonModel { return &LogRecord{} },
		func() commonModel { return &Event{} },
	}
	oddballs := []interface{}{
		&LogSegment{},
	}
	slices := []interface{}{
		&DependencyGraphs{},
		&CostReports{},
	}
	assert.NotNil(t, oddballs)
	assert.NotNil(t, slices)

	dbName := "sink_test"

	env := sink.GetEnvironment()
	assert.NoError(t, env.Configure(&sink.Configuration{
		MongoDBURI:   "mongodb://localhost:27017",
		NumWorkers:   2,
		DatabaseName: dbName,
	}))

	session, err := env.GetSession()
	require.NoError(t, err)
	require.NoError(t, session.DB(dbName).DropDatabase())

	defer func() {
		require.NoError(t, session.DB(dbName).DropDatabase())
	}()

	testCases := map[string]func(*testing.T, commonModel){
		"VerifyNillCheckIsCorrect": func(t *testing.T, m commonModel) {
			assert.True(t, m.IsNil())
		},
		"VerifySetEnv": func(t *testing.T, m commonModel) {
			assert.NotPanics(t, func() {
				m.Setup(sink.GetEnvironment())
			})
		},
		"FindErrorsWithoutEnv": func(t *testing.T, m commonModel) {
			err := m.Find()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "env is nil")
		},
		"FindErrorsForMissingDocument": func(t *testing.T, m commonModel) {
			m.Setup(sink.GetEnvironment())
			err := m.Find()
			assert.Error(t, err, "%+v", m)
		},
		"SaveErrorsForEmpty": func(t *testing.T, m commonModel) {
			m.Setup(sink.GetEnvironment())
			assert.Error(t, m.Save())
		},
	}

	for _, factory := range models {
		modelName := strings.Trim(fmt.Sprintf("%T", factory()), "*model.")
		t.Run(modelName, func(t *testing.T) {
			for name, test := range testCases {
				t.Run(name, func(t *testing.T) {
					test(t, factory())
				})
			}
		})
	}
}
