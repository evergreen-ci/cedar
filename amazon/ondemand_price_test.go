package amazon

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type OnDemandPriceSuite struct {
	suite.Suite
}

func TestOnDemandPriceSuite(t *testing.T) {
	suite.Run(t, new(OnDemandPriceSuite))
}

func (s *OnDemandPriceSuite) TestOSBillingName() {
	os := osBillingName("Windows (Amazon VPC)")
	s.Equal(os, "Windows")

	os = osBillingName("Linux/Unix")
	s.Equal(os, "Linux")
}

func (s *OnDemandPriceSuite) TestRegionFullName() {
	region, err := regionFullName("us-east-1")
	s.Equal(region, "US East (N. Virginia)")
	s.NoError(err)

	_, err = regionFullName("us-east-2")
	s.Error(err)
}

func (s *OnDemandPriceSuite) TestAZToRegion() {
	region := azToRegion("us-east-1b")
	s.Equal(region, "us-east-1")
}

func (s *OnDemandPriceSuite) TestGetOnDemandPriceInformation() {
	pricing, err := getOnDemandPriceInformation()
	s.NoError(err)
	s.NotZero(len(*pricing))
	count := 0
	for info, price := range *pricing {
		if count > 5 {
			break
		}
		count++
		res := (*pricing)[info]
		s.Equal(res, price)
	}
}

func (s *OnDemandPriceSuite) TestFetchPrice() {
	info := odInfo{
		os:       "Windows",
		instance: "c3.4xlarge",
		region:   "US East (N. Virginia)",
	}
	price := 1.2
	pricing := &prices{}
	(*pricing)[info] = price
	res := pricing.fetchPrice("Windows (Amazon VPC)", "c3.4xlarge", "us-east-1d")
	s.Equal(res, price)
}
