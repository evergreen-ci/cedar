package amazon

import (
	"math"

	"github.com/aws/aws-sdk-go/service/ec2"
)

// EBSPrices stores current EBS prices
type EBSPrices struct {
	GP2      float64 `yaml:"gp2"`
	IO1      float64 `yaml:"io1GB"`
	IO1IOPS  float64 `yaml:"io1IOPS"`
	ST1      float64 `yaml:"st1"`
	SC1      float64 `yaml:"sc1"`
	Standard float64 `yaml:"standard"`
	Snapshot float64 `yaml:"snapshot"`
}

func (p *EBSPrices) IsValid() bool {
	for _, val := range []float64{p.GP2, p.IO1, p.IO1IOPS, p.ST1, p.SC1, p.Standard, p.Snapshot} {
		if val != 0.0 {
			return true
		}
	}

	return false
}

func (pricing *EBSPrices) getPriceByVolumeType(vol *ec2.Volume, durationInDays float64) (float64, float64) {
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

func (pricing *EBSPrices) getEBSPrice(vol *ec2.Volume, reportRange TimeRange) float64 {
	uptimeStart := *vol.CreateTime
	// if report starts first, set the start of uptime to this
	if reportRange.Start.After(uptimeStart) {
		uptimeStart = reportRange.Start
	}
	denominator := 24.0 * 30.0 // (24 hours/day * 30 day-month)
	gbVol := float64(*vol.Size)
	duration := reportRange.End.Sub(uptimeStart)
	durationInDays := math.Ceil(duration.Hours() / 24.0)
	durationInHours := math.Ceil(duration.Hours())

	price, gbPrice := pricing.getPriceByVolumeType(vol, durationInDays)
	price += (gbPrice * gbVol * durationInHours) / denominator
	return roundUp(price, 2)
}
