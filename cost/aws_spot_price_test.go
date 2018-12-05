package cost

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/evergreen-ci/cedar/util"
	"github.com/stretchr/testify/suite"
)

type SpotPriceSuite struct {
	prices spotPrices
	suite.Suite
}

func TestSpotPriceSuite(t *testing.T) {
	suite.Run(t, new(SpotPriceSuite))
}

func (s *SpotPriceSuite) SetupTest() {
	spotPriceHistory := &ec2.DescribeSpotPriceHistoryOutput{}
	file, err := ioutil.ReadFile("testdata/spot-price-data.txt")
	if err != nil {
		fmt.Printf("Error reading file: %s\n", err)
	}

	err = json.Unmarshal(file, spotPriceHistory)
	if err != nil {
		fmt.Printf("Error unmarshalling file: %s\n", err)
	}
	s.prices = spotPrices(spotPriceHistory.SpotPriceHistory)
}

func (s *SpotPriceSuite) TestCalculatePriceOnePrice() {
	time1, _ := time.Parse(utcLayout, "2017-07-05T19:04:05.000Z")
	price1 := "0.84"
	block1 := &ec2.SpotPrice{
		Timestamp: &time1,
		SpotPrice: &price1,
	}
	start, _ := time.Parse(utcLayout, "2017-07-08T19:04:05.000Z")
	end, _ := time.Parse(utcLayout, "2017-07-08T22:04:05.000Z")
	var blocks = spotPrices([]*ec2.SpotPrice{block1})
	times := util.TimeRange{
		StartAt: start,
		EndAt:   end,
	}
	price := blocks.calculatePrice(times)
	s.Equal(price, 3*0.84)
}

func (s *SpotPriceSuite) TestCalculatePriceManyPrices() {
	time1, _ := time.Parse(utcLayout, "2017-07-05T19:04:05.000Z")
	time2, _ := time.Parse(utcLayout, "2017-07-08T20:04:05.000Z")
	time3, _ := time.Parse(utcLayout, "2017-07-08T21:19:05.000Z")
	start, _ := time.Parse(utcLayout, "2017-07-08T19:04:05.000Z")
	end, _ := time.Parse(utcLayout, "2017-07-08T22:04:05.000Z")
	price1 := "0.84"
	price2 := "1.00"
	price3 := "0.5"
	// Note this first time is 3 days before the report starts
	block1 := &ec2.SpotPrice{
		Timestamp: &time1, // should be one hour
		SpotPrice: &price1,
	}
	block2 := &ec2.SpotPrice{
		Timestamp: &time2, //should be 1 hour and 15 minutes
		SpotPrice: &price2,
	}
	block3 := &ec2.SpotPrice{
		Timestamp: &time3, // should be 45 minutes
		SpotPrice: &price3,
	}
	var blocks = spotPrices([]*ec2.SpotPrice{block1, block3, block2})
	times := util.TimeRange{
		StartAt: start,
		EndAt:   end,
	}
	price := blocks.calculatePrice(times)
	expectedPrice := 0.84 + 1.25 + (0.5 * 0.75)
	s.Equal(price, expectedPrice)
}

func (s *SpotPriceSuite) TestCalculateRealPrices() {
	s.T().Skip("integration test dependent change")
	time1, _ := time.Parse(utcLayout, "2018-08-15T06:19:00.000Z")
	time2, _ := time.Parse(utcLayout, "2018-08-15T10:19:00.000Z")
	times := util.TimeRange{
		StartAt: time1,
		EndAt:   time2,
	}
	price := s.prices.calculatePrice(times)
	s.Equal(0.17559225, price)
}
