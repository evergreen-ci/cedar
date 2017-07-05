package amazon

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/stretchr/testify/suite"
)

type SpotPriceSuite struct {
	suite.Suite
}

func TestSpotPriceSuite(t *testing.T) {
	suite.Run(t, new(SpotPriceSuite))
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
	times := TimeRange{
		Start: start,
		End:   end,
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
	times := TimeRange{
		Start: start,
		End:   end,
	}
	price := blocks.calculatePrice(times)
	expectedPrice := 0.84 + 1.25 + (0.5 * 0.75)
	s.Equal(price, expectedPrice)
}
