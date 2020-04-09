package cost

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/evergreen-ci/cedar/model"
	"github.com/stretchr/testify/suite"
)

type EBSPriceSuite struct {
	suite.Suite
	reportRange model.TimeRange
	ebsPrices   *model.CostConfigAmazonEBS
	volume      *ec2.Volume
}

func TestEBSPriceSuite(t *testing.T) {
	suite.Run(t, new(EBSPriceSuite))
}

func (s *EBSPriceSuite) SetupSuite() {
	s.ebsPrices = &model.CostConfigAmazonEBS{
		GP2:      0.10,
		IO1:      0.125,
		IO1IOPS:  0.065,
		ST1:      0.045,
		SC1:      0.025,
		Standard: 0.05,
		Snapshot: 0.05,
	}
	createTime, _ := time.Parse(utcLayout, "2017-07-05T06:04:05.000Z")
	time1, _ := time.Parse(utcLayout, "2017-07-05T07:04:05.000Z")
	time2, _ := time.Parse(utcLayout, "2017-07-05T19:04:05.000Z")

	s.reportRange = model.TimeRange{
		StartAt: time1,
		EndAt:   time2,
	}
	size := int64(2000)
	iops := int64(1000)
	s.volume = &ec2.Volume{
		CreateTime: &createTime,
		Size:       &size,
		Iops:       &iops,
	}
}

func (s *EBSPriceSuite) TestGetEBSPriceWithEBSVolumeTypeIO1() {
	s.volume.SetVolumeType(ec2.VolumeTypeIo1)
	price := getEBSPrice(*s.ebsPrices, s.volume, s.reportRange)
	actualPrice := 4.17 + 2.17
	s.Equal(price, actualPrice)
}

func (s *EBSPriceSuite) TestGetEBSPriceWithEBSVolumeTypeST1() {
	s.volume.SetVolumeType(ec2.VolumeTypeSt1)
	price := getEBSPrice(*s.ebsPrices, s.volume, s.reportRange)
	actualPrice := 1.50
	s.Equal(price, actualPrice)
}

func (s *EBSPriceSuite) TestGetEBSPriceWithEBSVolumeTypeStandard() {
	s.volume.SetVolumeType(ec2.VolumeTypeStandard)
	price := getEBSPrice(*s.ebsPrices, s.volume, s.reportRange)
	actualPrice := 1.67 + 0.05

	s.Equal(price, actualPrice)
}
