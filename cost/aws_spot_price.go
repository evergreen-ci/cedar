package cost

import (
	"math"
	"sort"
	"strconv"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/evergreen-ci/cedar/util"
)

type spotPrices []*ec2.SpotPrice

// The following spotPrices methods are to implement sort.Sort.
func (slice spotPrices) Len() int {
	return len(slice)
}

func (slice spotPrices) Less(i, j int) bool {
	return (*slice[i].Timestamp).After(*slice[j].Timestamp)
}

func (slice spotPrices) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

// calculatePrice takes in a TimeRange and returns the price for this range
// using the given spotPrices.
func (slice spotPrices) calculatePrice(times util.TimeRange) float64 {
	price := 0.0
	computed := 0.0
	totalTime := times.EndAt.Sub(times.StartAt).Hours()
	lastTime := times.EndAt

	// sorts slice in descending time order
	sort.Sort(slice)

	for _, block := range slice {
		available := lastTime.Sub(*block.Timestamp).Hours()
		if available < 0 {
			// error in API results
			return 0
		}

		remaining := totalTime - computed
		used := math.Min(available, remaining)
		blockPrice, err := strconv.ParseFloat(*block.SpotPrice, 64)
		if err != nil {
			return 0.0
		}
		price += blockPrice * used
		computed += used
		lastTime = *block.Timestamp
	}
	return price
}
