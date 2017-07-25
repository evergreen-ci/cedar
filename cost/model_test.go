package cost

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func createTestStruct() Output {
	var cost Output
	report1 := Report{
		Generated: "2017-05-23T12:11:10.123",
		Begin:     "2017-05-23T17:00:00.000Z",
		End:       "2017-05-23T17:12:00.000Z",
	}

	item1 := &Item{
		Name:       "c3.4xlarge",
		ItemType:   "spot",
		Launched:   12,
		Terminated: 1,
		AvgPrice:   0.704,
		AvgUptime:  1.62,
		TotalHours: 23,
	}

	service1 := &Service{
		Name:  "ec2",
		Items: []*Item{item1},
	}
	service2 := &Service{
		Name: "ebs",
		Cost: 5000,
	}
	service3 := &Service{
		Name: "s3",
		Cost: 5000,
	}

	account1 := &Account{
		Name:     "kernel-build",
		Services: []*Service{service1, service2, service3},
	}

	provider1 := &Provider{
		Name:     "aws",
		Accounts: []*Account{account1},
	}
	provider2 := &Provider{
		Name: "macstadium",
		Cost: 27.12,
	}

	task1 := &Task{
		Githash:      "c609be45647fce98d0394221efc5d362ac470b64",
		Name:         "compile",
		Distro:       "ubuntu1604-build",
		BuildVariant: "x...",
		TaskSeconds:  1242,
	}

	project1 := &Project{
		Name:  "mongodb-mongo-master",
		Tasks: []*Task{task1},
	}
	distro1 := &Distro{
		Name:            "ubuntu1604-build",
		Provider:        "ec2",
		InstanceType:    "c3.4xlarge",
		InstanceSeconds: 12,
	}

	evergreen1 := Evergreen{
		Projects: []*Project{project1},
		Distros:  []*Distro{distro1},
	}
	cost = Output{
		Report:    report1,
		Evergreen: evergreen1,
		Providers: []*Provider{provider1, provider2},
	}

	return cost
}

//Verify that Output struct can be converted to and from JSON.
func TestModelStructToJSON(t *testing.T) {
	assert := assert.New(t)
	var costFromJSON Output
	cost := createTestStruct()
	raw, err := json.Marshal(cost)
	assert.NoError(err)
	json.Unmarshal(raw, &costFromJSON)
	assert.Equal(costFromJSON, cost)
}
