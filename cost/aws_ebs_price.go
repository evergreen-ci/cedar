package cost

import (
	"math"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/util"
)

func getPriceByVolumeType(pricing model.CostConfigAmazonEBS, vol *ec2.Volume, durationInDays float64) (float64, float64) {
	var volumePrice, price float64
	onemillion := 1000000.0
	switch *vol.VolumeType {
	case ec2.VolumeTypeIo1:
		volumePrice = pricing.IO1
		if vol.Iops != nil {
			iops := float64(*vol.Iops)
			price = (pricing.IO1IOPS * iops * durationInDays) / 30.0
		}
	case ec2.VolumeTypeGp2:
		volumePrice = pricing.GP2
	case ec2.VolumeTypeSt1:
		volumePrice = pricing.ST1
	case ec2.VolumeTypeSc1:
		volumePrice = pricing.SC1
	default:
		volumePrice = pricing.Standard
		if vol.Iops != nil {
			iops := float64(*vol.Iops)
			millionsOfIOPS := math.Ceil(iops / onemillion)
			price = pricing.Standard * millionsOfIOPS
		}
	}
	return price, volumePrice
}

func getEBSPrice(pricing model.CostConfigAmazonEBS, vol *ec2.Volume, reportRange util.TimeRange) float64 {
	uptimeStart := *vol.CreateTime
	// if report starts first, set the start of uptime to this
	if reportRange.StartAt.After(uptimeStart) {
		uptimeStart = reportRange.StartAt
	}
	denominator := 24.0 * 30.0 // (24 hours/day * 30 day-month)
	gbVol := float64(*vol.Size)
	duration := reportRange.EndAt.Sub(uptimeStart)
	durationInDays := math.Ceil(duration.Hours() / 24.0)
	durationInHours := math.Ceil(duration.Hours())

	price, gbPrice := getPriceByVolumeType(pricing, vol, durationInDays)
	price += (gbPrice * gbVol * durationInHours) / denominator
	return util.RoundUp(price, 2)
}
