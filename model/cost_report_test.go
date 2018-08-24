package model

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/evergreen-ci/sink"
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env := sink.GetEnvironment()
	require.NoError(t, env.Configure(&sink.Configuration{
		MongoDBURI:    "mongodb://localhost:27017",
		DatabaseName:  "sink.test.costreport",
		NumWorkers:    2,
		UseLocalQueue: true,
	}))

	defer func() {
		conf, session, err := sink.GetSessionWithConfig(env)
		require.NoError(t, err)
		if err := session.DB(conf.DatabaseName).DropDatabase(); err != nil {
			assert.Contains(t, err.Error(), "not found")
		}
	}()

	for name, test := range map[string]func(context.Context, *testing.T, sink.Environment, *CostReport){
		// "": func(ctx context.Context, t *testing.t, env sink.Environment, conf *Costreport) {},
	} {
		t.Run(name, func(t *testing.T) {
			tctx, cancel := context.WithCancel(ctx)
			defer cancel()
			test(tctx, t, env, &CostReport{})
		})
	}
}
