package util

import "math"

// RoundUp rounds the input number up, with places representing the number of decimal places.
func RoundUp(input float64, places int) float64 {
	var round float64
	pow := math.Pow(10, float64(places))
	digit := pow * input
	round = math.Ceil(digit)
	newVal := round / pow
	return newVal
}

// Average returns the average of the vals.
func Average(vals []float64) float64 {
	total := 0.0
	for _, v := range vals {
		total += v
	}
	avg := total / float64(len(vals))
	return RoundUp(avg, 2)
}

func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}

	return false
}

//SumInt64 returns the sum of the vals.
func SumInt64(vals []int64) int64 {
	sum := int64(0)
	for _, val := range vals {
		sum += val
	}
	return sum
}
