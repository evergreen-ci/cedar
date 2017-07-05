package cost

import (
	"testing"
	"time"

	"github.com/evergreen-ci/sink/amazon"
	"github.com/mongodb/grip"
	"github.com/stretchr/testify/suite"
)

func init() {
	grip.SetName("sink.cost.test")
}

// CommandsSuite provide a group of tests for the cost helper functions.
type CostSuite struct {
	suite.Suite
	config *Config
}

func TestServiceCacheSuite(t *testing.T) {
	suite.Run(t, new(CostSuite))
}

func (c *CostSuite) SetupTest() {
	c.config = &Config{Opts: Options{}, Providers: []*Provider{}}
}

func (c *CostSuite) TestGetGranularity() {
	c.config.Opts.Duration = "0s"
	granularity, err := c.config.GetGranularity() //zero Duration
	c.NoError(err)
	c.Equal(granularity.String(), "4h0m0s")
	c.config.Opts.Duration = "12h"
	granularity, err = c.config.GetGranularity()
	c.NoError(err)
	c.Equal(granularity.String(), "12h0m0s")
}

func (c *CostSuite) TestUpdateSpendProviders() {
	oldProv1 := &Provider{
		Name: "Provider1",
		Cost: 1234,
	}
	oldProv2 := &Provider{
		Name: "Provider2",
		Cost: 4200,
	}
	newProv1 := &Provider{
		Name: "Provider2",
		Cost: 1200,
	}
	newProv2 := &Provider{
		Name: "Provider3",
		Cost: 15251,
	}
	newProv := []*Provider{newProv1, newProv2}
	c.config.Providers = []*Provider{oldProv1, oldProv2}
	c.config.UpdateSpendProviders(newProv)
	result := c.config.Providers

	c.Len(result, 3)
	c.EqualValues(result[0], oldProv1) //Shouldn't change
	c.EqualValues(result[1], newProv1) //Should change
	c.EqualValues(result[2], newProv2)
}

func (c *CostSuite) TestGetStartTime() {
	start := "2017-05-26T12:00"
	granularity := 4 * time.Hour

	startTime, err := GetStartTime(start, granularity)
	c.NoError(err)
	str := startTime.String()
	c.Equal(str[0:10], start[0:10])   //YYYY-MM-DD
	c.Equal(str[11:16], start[11:16]) //HH:MM

	_, err = GetStartTime("", granularity)
	c.NoError(err)

	_, err = GetStartTime("2011T00:00", granularity)
	c.Error(err)
}

func (c *CostSuite) TestYAMLToConfig() {
	file := "testdata/spend_test.yml"
	_, err := YAMLToConfig(file)
	c.NoError(err)

	file = "not_real.yaml"
	_, err = YAMLToConfig(file)
	c.Error(err)
}

func (c *CostSuite) TestCreateItemFromEC2Instance() {
	key := &amazon.ItemKey{
		ItemType: "reserved",
		Name:     "c3.4xlarge",
	}
	item1 := &amazon.EC2Item{
		Launched:   true,
		Terminated: false,
		FixedPrice: 120.0,
		Price:      12.42,
		Uptime:     3.0,
		Count:      5,
	}
	item2 := &amazon.EC2Item{
		Launched:   false,
		Terminated: true,
		FixedPrice: 35.0,
		Price:      0.33,
		Uptime:     1.00,
		Count:      2,
	}
	item3 := &amazon.EC2Item{
		Launched:   true,
		Terminated: false,
		FixedPrice: 49.0,
		Price:      0.5,
		Uptime:     1.00,
		Count:      1,
	}
	items := []*amazon.EC2Item{item1, item2, item3}
	item := createItemFromEC2Instance(key, items)
	c.Equal(item.Name, key.Name)
	c.Equal(item.ItemType, string(key.ItemType))
	c.Equal(item.AvgPrice, float32(4.42))
	c.Equal(item.FixedPrice, float32(68))
	c.Equal(item.AvgUptime, float32(1.67))
	c.Equal(item.Launched, 6)
	c.Equal(item.Terminated, 2)
}
