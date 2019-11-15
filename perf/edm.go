package perf

import (
	"math"
	"sort"
)

// NewEdmDetector calculates changepoints for the series, using
// e-divisive-with-medians. The change points returned do not include
// windowing information.
func NewEDMDetector(minSize int) ChangeDetector {
	return &edmDetector{
		minSize: minSize,
		info: AlgorithmInfo{
			Name:    "e_divisive_with_medians",
			Version: 1,
			Options: []AlgorithmOption{
				{
					Name:  "minSize",
					Value: minSize,
				},
			},
		},
	}
}

type edmDetector struct {
	minSize int
	info    AlgorithmInfo
}

func (d edmDetector) eDivisiveWithMedians(series []float64) []int {
	n := len(series)
	prev := make([]int, n+1, n+1)
	number := make([]int, n+1, n+1)
	F := make([]float64, n+1, n+1)
	for i := range F {
		// Jim, why is this -3?
		F[i] = -3.0
	}

	right := &sortedList{}
	left := &sortedList{}
	for s := 2 * d.minSize; s < n+1; s++ {
		right.Clear()
		left.Clear()
		left.Insert(series[prev[d.minSize-1] : d.minSize-1]...)
		right.Insert(series[d.minSize-1 : s]...)
		for t := d.minSize; t < s-d.minSize+1; t++ { //modify limits to deal with minSize
			left.Insert(series[t-1])
			right.Remove(series[t-1])

			if prev[t] > prev[t-1] {
				for i := prev[t-1]; i < prev[t]; i++ {
					left.Remove(series[i])
				}
			} else if prev[t] < prev[t-1] {
				for i := prev[t]; i < prev[t-1]; i++ {
					left.Insert(series[i])
				}
			}

			//calculate statistic value
			leftMedian := left.Median()
			rightMedian := right.Median()

			normalize := float64((t-prev[t])*(s-t)) / math.Pow(float64(s-prev[t]), 2.0)
			tmp := F[t] + normalize*math.Pow(leftMedian-rightMedian, 2.0)

			//check for improved optimal statistic value
			if tmp > F[s] {
				number[s] = number[t] + 1
				F[s] = tmp
				prev[s] = t
			}
		}
	}

	loc := make([]int, 0, len(prev))

	for at := n; at > 0; at = prev[at] {
		value := prev[at]
		if value != 0 {
			loc = append(loc, value)
		}
	}
	sort.Slice(loc, func(i, j int) bool {
		return loc[i] < loc[j]
	})
	return loc
}

func (d edmDetector) DetectChanges(series []float64) ([]ChangePoint, error) {
	results := d.eDivisiveWithMedians(series)
	out := make([]ChangePoint, 0, len(results))
	for _, r := range results {
		out = append(out, ChangePoint{
			Index: r,
			Info:  d.info,
		})
	}
	return out, nil
}
