package model

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const timeLayout = "2006-01-02T15:04:05.000"

func createTestStruct() CostReport {
	var (
		cost CostReport
		err  error
	)
	report1 := CostReportMetadata{}

	report1.Generated, err = time.Parse(timeLayout, "2017-05-23T12:11:10.123")
	if err != nil {
		panic(err.Error())
	}
	report1.Range.StartAt, err = time.Parse(timeLayout, "2017-05-23T17:00:00.000")
	if err != nil {
		panic(err.Error())
	}
	report1.Range.EndAt, err = time.Parse(timeLayout, "2017-05-23T17:12:00.000")
	if err != nil {
		panic(err.Error())
	}

	item1 := ServiceItem{
		Name:       "c3.4xlarge",
		ItemType:   "spot",
		Launched:   12,
		Terminated: 1,
		AvgPrice:   0.704,
		AvgUptime:  1.62,
		TotalHours: 23,
	}

	service1 := AccountService{
		Name:  "ec2",
		Items: []ServiceItem{item1},
	}
	service2 := AccountService{
		Name: "ebs",
		Cost: 5000,
	}
	service3 := AccountService{
		Name: "s3",
		Cost: 5000,
	}

	account1 := CloudAccount{
		Name:     "kernel-build",
		Services: []AccountService{service1, service2, service3},
	}

	provider1 := CloudProvider{
		Name:     "aws",
		Accounts: []CloudAccount{account1},
	}
	provider2 := CloudProvider{
		Name: "macstadium",
		Cost: 27.12,
	}

	task1 := EvergreenTaskCost{
		Githash:      "c609be45647fce98d0394221efc5d362ac470b64",
		Name:         "compile",
		Distro:       "ubuntu1604-build",
		BuildVariant: "x...",
		TaskSeconds:  1242,
	}

	project1 := EvergreenProjectCost{
		Name:  "mongodb-mongo-master",
		Tasks: []EvergreenTaskCost{task1},
	}
	distro1 := EvergreenDistroCost{
		Name:            "ubuntu1604-build",
		Provider:        "ec2",
		InstanceType:    "c3.4xlarge",
		InstanceSeconds: 12,
	}

	evergreen1 := EvergreenCost{
		Projects: []EvergreenProjectCost{project1},
		Distros:  []EvergreenDistroCost{distro1},
	}
	cost = CostReport{
		Report:    report1,
		Evergreen: evergreen1,
		Providers: []CloudProvider{provider1, provider2},
	}

	return cost
}

//Verify that Output struct can be converted to and from JSON.
func TestModelStructToJSON(t *testing.T) {
	assert := assert.New(t)
	var costFromJSON CostReport
	cost := createTestStruct()
	raw, err := json.Marshal(cost)
	assert.NoError(err)
	assert.NoError(json.Unmarshal(raw, &costFromJSON))
	assert.Equal(costFromJSON, cost)
}

func TestCostReport(t *testing.T) {
	cleanup := func() {
		conf, session, err := cedar.GetSessionWithConfig(cedar.GetEnvironment())
		require.NoError(t, err)
		if err := session.DB(conf.DatabaseName).DropDatabase(); err != nil {
			assert.Contains(t, err.Error(), "not found")
		}
	}

	defer cleanup()

	for name, test := range map[string]func(context.Context, *testing.T, cedar.Environment, *CostReport){
		"VerifyFixtures": func(ctx context.Context, t *testing.T, env cedar.Environment, report *CostReport) {
			assert.NotNil(t, env)
			assert.NotNil(t, report)
			assert.True(t, report.IsNil())
		},
		"FindErrorsWithoutReportig": func(ctx context.Context, t *testing.T, env cedar.Environment, report *CostReport) {
			assert.Error(t, report.Find())
		},
		"FindErrorsWithNoResults": func(ctx context.Context, t *testing.T, env cedar.Environment, report *CostReport) {
			report.Setup(env)
			err := report.Find()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "could not find")
		},
		"FindErrorsWthBadDbName": func(ctx context.Context, t *testing.T, _ cedar.Environment, report *CostReport) {
			env, err := cedar.NewEnvironment(ctx, "broken", &cedar.Configuration{
				MongoDBURI:         "mongodb://localhost:27017",
				DatabaseName:       "\"", // intentionally invalid
				NumWorkers:         2,
				DisableRemoteQueue: true,
			})
			require.NoError(t, err)

			report.Setup(env)
			err = report.Find()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "problem finding")
		},
		"SimpleRoundTrip": func(ctx context.Context, t *testing.T, env cedar.Environment, report *CostReport) {
			t.Skip("FIX ME")
			report.Setup(env)
			assert.NoError(t, report.Save())
			err := report.Find()
			assert.NoError(t, err)
		},
		"SaveErrorsWithBadDBName": func(ctx context.Context, t *testing.T, _ cedar.Environment, report *CostReport) {
			env, err := cedar.NewEnvironment(ctx, "broken", &cedar.Configuration{
				MongoDBURI:         "mongodb://localhost:27017",
				DatabaseName:       "\"", // intentionally invalid
				NumWorkers:         2,
				DisableRemoteQueue: true,
			})
			require.NoError(t, err)

			report.ID = "one"
			report.Setup(env)
			report.populated = true
			err = report.Save()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "problem saving cost reporting")
		},
		"SaveErrorsWithNoEnvConfigured": func(ctx context.Context, t *testing.T, env cedar.Environment, report *CostReport) {
			report.ID = "two"
			report.populated = true
			err := report.Save()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "env is nil")
		},
		"MockedInternalCache": func(ctx context.Context, t *testing.T, env cedar.Environment, report *CostReport) {
			report.Providers = append(report.Providers, CloudProvider{
				Name: "one",
				Cost: .42,
				Accounts: []CloudAccount{
					{
						Name: "two",
						Cost: .84,
						Services: []AccountService{
							{
								Name: "two",
								Cost: 1.22,
							},
						},
					},
				},
			})
			assert.Len(t, report.providers, 0)

			report.refresh()
			assert.Len(t, report.providers, 1)

		},
		"StringFormIsJson": func(ctx context.Context, t *testing.T, env cedar.Environment, report *CostReport) {
			str := report.String()
			assert.Equal(t, string(str[0]), "{")
			assert.Equal(t, string(str[len(str)-1]), "}")
		},
		"FindReturnsDocument": func(ctx context.Context, t *testing.T, env cedar.Environment, report *CostReport) {
			report.Setup(env)
			report.ID = "test_doc"
			report.Report.Generated = time.Now().Round(time.Millisecond)
			assert.NoError(t, report.Save())

			r2 := &CostReport{ID: "test_doc"}
			r2.Setup(env)
			assert.False(t, r2.populated)
			assert.NoError(t, r2.Find())
			assert.True(t, r2.populated)
			assert.Equal(t, report.Report.Generated.UTC(), r2.Report.Generated.UTC())
		},
		"DataCachingWithMockDocument": func(ctx context.Context, t *testing.T, env cedar.Environment, _ *CostReport) {
			r := createTestStruct()

			assert.Len(t, r.providers, 0)
			assert.Len(t, r.Evergreen.projects, 0)
			r.refresh()
			assert.Len(t, r.providers, 2)
			assert.Len(t, r.Evergreen.projects, 1)
		},
		// "": func(ctx context.Context, t *testing.T, env cedar.Environment, report *CostReport) {},
	} {
		t.Run(name, func(t *testing.T) {
			cleanup()

			env := cedar.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()

			test(ctx, t, env, &CostReport{})
		})
	}
}

func TestCostDataStructures(t *testing.T) {
	t.Run("ServiceItemID", func(t *testing.T) {
		item := &ServiceItem{
			Name:     "name",
			ItemType: "foo",
		}
		assert.Equal(t, "name", item.ID())
		item.Name = ""
		assert.Equal(t, "foo", item.ID())
	})

	for name, test := range map[string]func(*testing.T, *ServiceItem){
		"CachesCostReturnedAppropriatly": func(t *testing.T, item *ServiceItem) {
			item.TotalCost = 1.0
			assert.Equal(t, 1.0, item.GetCost(util.TimeRange{}))
		},
		"FixedPricesForTime": func(t *testing.T, item *ServiceItem) {
			item.TotalHours = 1
			item.FixedPrice = 2
			assert.Equal(t, 2.0, item.GetCost(util.TimeRange{}))
		},
		"AveragePriceForFixedTime": func(t *testing.T, item *ServiceItem) {
			item.TotalHours = 1
			item.AvgPrice = 3
			assert.Equal(t, 3.0, item.GetCost(util.TimeRange{}))
		},
		"DefinedAverageUptimeFixedPrice": func(t *testing.T, item *ServiceItem) {
			item.AvgUptime = 10
			item.FixedPrice = 10
			assert.Equal(t, 100.0, item.GetCost(util.TimeRange{}))
		},
		"DefinedAverageUptimeAveragePrice": func(t *testing.T, item *ServiceItem) {
			item.AvgUptime = 10
			item.AvgPrice = 3
			assert.Equal(t, 30.0, item.GetCost(util.TimeRange{}))
		},
		"UnspecifiedAverageUptimeFixedPrices": func(t *testing.T, item *ServiceItem) {
			item.FixedPrice = 5
			item.Launched = 3
			item.Terminated = 0
			tr := util.TimeRange{StartAt: time.Now(), EndAt: time.Now().Add(time.Hour + time.Second)}
			assert.InDelta(t, 15.0, item.GetCost(tr), 0.1)
		},
		"UnspecifiedAverageUptimeAveragePrices": func(t *testing.T, item *ServiceItem) {
			item.AvgPrice = 8
			item.Launched = 2
			item.Terminated = 0
			tr := util.TimeRange{StartAt: time.Now(), EndAt: time.Now().Add(time.Hour + time.Second)}
			assert.InDelta(t, 16.0, item.GetCost(tr), 0.1)
		},
		"UnspecifiedButNoReportDuration": func(t *testing.T, item *ServiceItem) {
			item.AvgPrice = 10
			item.Launched = 3
			item.Terminated = 0
			assert.Equal(t, 0.0, item.GetCost(util.TimeRange{}))
		},
		"UnspecifiedEmptyNumberOfItems": func(t *testing.T, item *ServiceItem) {
			item.AvgPrice = 10
			item.Launched = 0
			item.Terminated = -1
			tr := util.TimeRange{StartAt: time.Now(), EndAt: time.Now().Add(time.Hour)}
			assert.InDelta(t, 10.0, item.GetCost(tr), .01)
		},
		"TotalHousSpecifiedButNoPrice": func(t *testing.T, item *ServiceItem) {
			item.TotalHours = 4
			assert.Equal(t, 0.0, item.GetCost(util.TimeRange{}))
		},
		"IsZeroForZeroTypes": func(t *testing.T, item *ServiceItem) {
			assert.Equal(t, 0.0, item.GetCost(util.TimeRange{}))
		},
		// "": func(t *testing.T, item *ServiceItem) {},
	} {
		t.Run(name, func(t *testing.T) {
			test(t, &ServiceItem{})
		})
	}
}
