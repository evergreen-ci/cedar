package cost

import "math"

// roundUp rounds the input number up, with places representing the
// number of decimal places.
func roundUp(input float64, places int) float64 {
	var round float64
	pow := math.Pow(10, float64(places))
	digit := pow * input
	round = math.Ceil(digit)
	newVal := round / pow
	return newVal
}

// average returns the average of the vals.
func average(vals []float64) float64 {
	total := 0.0
	for _, v := range vals {
		total += v
	}
	avg := total / float64(len(vals))
	return roundUp(avg, 2)
}
