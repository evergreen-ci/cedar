package model

import (
	"context"
	"testing"

	"github.com/evergreen-ci/cedar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCostReportSummary(t *testing.T) {
	cleanup := func() {
		env := cedar.GetEnvironment()

		conf, session, err := cedar.GetSessionWithConfig(env)
		require.NoError(t, err)
		if err := session.DB(conf.DatabaseName).DropDatabase(); err != nil {
			assert.Contains(t, err.Error(), "not found")
		}
	}

	defer cleanup()

	for name, test := range map[string]func(context.Context, *testing.T, cedar.Environment, *CostReportSummary){
		"VerifyFixtures": func(ctx context.Context, t *testing.T, env cedar.Environment, report *CostReportSummary) {
			assert.NotNil(t, env)
			assert.NotNil(t, report)
			assert.True(t, report.IsNil())
		},
		"FindErrorsWithoutReportig": func(ctx context.Context, t *testing.T, env cedar.Environment, report *CostReportSummary) {
			assert.Error(t, report.Find())
		},
		"FindErrorsWithNoResults": func(ctx context.Context, t *testing.T, env cedar.Environment, report *CostReportSummary) {
			report.Setup(env)
			err := report.Find()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "could not find")
		},
		"FindErrorsWthBadDbName": func(ctx context.Context, t *testing.T, _ cedar.Environment, report *CostReportSummary) {
			env, err := cedar.NewEnvironment(ctx, "broken", &cedar.Configuration{
				MongoDBURI:              "mongodb://localhost:27017",
				DatabaseName:            "\"", // intentionally invalid
				NumWorkers:              2,
				DisableRemoteQueue:      true,
				DisableRemoteQueueGroup: true,
			})
			assert.NoError(t, err)

			report.Setup(env)
			err = report.Find()

			assert.Error(t, err)
			assert.Contains(t, err.Error(), "problem finding")
		},
		"SimpleRoundTrip": func(ctx context.Context, t *testing.T, env cedar.Environment, report *CostReportSummary) {
			t.Skip("FIX ME")
			report.Setup(env)
			assert.NoError(t, report.Save())
			err := report.Find()
			assert.NoError(t, err)
		},
		"SaveErrorsWithBadDBName": func(ctx context.Context, t *testing.T, _ cedar.Environment, report *CostReportSummary) {
			env, err := cedar.NewEnvironment(ctx, "broken", &cedar.Configuration{
				MongoDBURI:              "mongodb://localhost:27017",
				DatabaseName:            "\"", // intentionally invalid
				NumWorkers:              2,
				DisableRemoteQueue:      true,
				DisableRemoteQueueGroup: true,
			})
			require.NoError(t, err)

			report.ID = "one"
			report.Setup(env)
			report.populated = true
			err = report.Save()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "Invalid namespace")
		},
		"SaveErrorsWithNoEnvConfigured": func(ctx context.Context, t *testing.T, env cedar.Environment, report *CostReportSummary) {
			report.ID = "two"
			report.populated = true
			err := report.Save()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "env is nil")
		},
		"StringFormIsJson": func(ctx context.Context, t *testing.T, env cedar.Environment, report *CostReportSummary) {
			str := report.String()
			assert.Equal(t, string(str[0]), "{")
			assert.Equal(t, string(str[len(str)-1]), "}")
		},
		"FindReturnsDocument": func(ctx context.Context, t *testing.T, env cedar.Environment, report *CostReportSummary) {
			report.Setup(env)
			report.ID = "test_doc"
			report.populated = true
			assert.NoError(t, report.Save())

			r2 := &CostReportSummary{ID: "test_doc"}
			r2.Setup(env)
			assert.False(t, r2.populated)
			assert.NoError(t, r2.Find())
			assert.True(t, r2.populated)
		},
	} {
		t.Run(name, func(t *testing.T) {
			env := cedar.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()
			test(ctx, t, env, &CostReportSummary{})
		})
	}
}
