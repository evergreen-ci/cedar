package perf

import (
	"math"
	"math/rand"
	"sort"
)

// ChangePoint represents a single change point.
type qhatChangePoint struct {
	Index       int
	Q           float64
	Probability float64
	Order       int
	qhatWindow
}

type qhatWindow struct {
	Start int
	End   int
}

type qhatDetector struct {
	rand         *rand.Rand
	pvalue       float64
	permutations int
	info         AlgorithmInfo
}

// NewQHatDetector uses the e-divisive (q^) algorithm to return change
// points for series.
func NewQHatDetector(pvalue float64, permutations int, seed int64) ChangeDetector {
	return &qhatDetector{
		rand:         rand.New(rand.NewSource(seed)),
		pvalue:       pvalue,
		permutations: permutations,
		info: AlgorithmInfo{
			Name:    "e_divisive",
			Version: 1,
			Options: []AlgorithmOption{
				{
					Name:  "p",
					Value: pvalue,
				},
				{
					Name:  "permutations",
					Value: permutations,
				},
			},
		},
	}
}

func (qhatDetector) calculateDiffs(series []float64) []float64 {
	length := len(series)
	diffs := make([]float64, length*length, length*length)
	for row := 0; row < length; row++ {
		for column := row; column < length; column++ {
			delta := math.Abs(series[row] - series[column])
			diffs[row*length+column] = delta
			diffs[column*length+row] = delta
		}
	}
	return diffs
}

func (qhatDetector) calculateQ(term1 float64, term2 float64, term3 float64, suffix int, prefix int) float64 {
	m := float64(suffix)
	n := float64(prefix)

	term1Reg := term1 * (2.0 / (m * n))
	term2Reg := term2 * (2.0 / (n * (n - 1)))
	term3Reg := term3 * (2.0 / (m * (m - 1)))
	newq := float64(int((m * n) / (m + n)))
	newq = newq * (term1Reg - term2Reg - term3Reg)
	return newq
}

func (d *qhatDetector) qHat(series []float64) []float64 {
	length := len(series)
	qhatValues := make([]float64, length)

	if length >= 5 {
		diffs := d.calculateDiffs(series)

		n := 2
		m := length - n

		term1 := 0.0
		for i := 0; i < n; i++ {
			for j := n; j < length; j++ {
				term1 += diffs[i*length+j]
			}
		}
		term2 := 0.0
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				term2 += diffs[i*length+j]
			}
		}

		term3 := 0.0
		for i := n; i < length; i++ {
			for j := i + 1; j < length; j++ {
				term3 += diffs[i*length+j]
			}

		}

		qhatValues[n] = d.calculateQ(term1, term2, term3, m, n)

		for n := 3; n < (length - 2); n++ {
			m = length - n
			rowDelta := 0.0

			for j := 0; j < n-1; j++ {
				rowDelta = rowDelta + diffs[(n-1)*length+j]
			}

			columnDelta := 0.0
			for j := n - 1; j < length; j++ {
				columnDelta = columnDelta + diffs[j*length+n-1]
			}

			term1 = term1 - rowDelta + columnDelta
			term2 = term2 + rowDelta
			term3 = term3 - columnDelta

			qhatValues[n] = d.calculateQ(term1, term2, term3, m, n)
		}
	}
	return qhatValues
}

func (qhatDetector) extractQ(qhatValues []float64) (int, float64) {
	var (
		index int
		value float64
	)

	length := len(qhatValues)
	for i := 0; i < length; i++ {
		if qhatValues[i] > value {
			index = i
			value = qhatValues[i]
		}
	}

	return index, value
}

// Shuffle a copy of each window, get the highest Q.
// Return true if ay of the permutations are greater than q.
func (d *qhatDetector) shuffleWindows(series []float64, windows []qhatWindow, q float64) bool {
	series = append([]float64{}, series...)
	maxQ := -1.0
	for _, w := range windows {
		window := series[w.Start:w.End]
		d.rand.Shuffle(len(window), func(i, j int) { window[i], window[j] = window[j], window[i] })

		winQs := d.qHat(window)
		if _, winMaxQs := d.extractQ(winQs); winMaxQs > maxQ {
			maxQ = winMaxQs
		}
	}
	return maxQ >= q
}

// Create Windows, if there are none create a window that encompasses the whole range. If there are
// some windows then split the window that encompasses index and resort.
func (qhatDetector) createWindows(windows []qhatWindow, index int, length int) ([]qhatWindow, int) {
	found := -1

	if len(windows) == 0 {
		windows = append(windows, qhatWindow{Start: 0, End: length})
	} else {
		for i, current := range windows {
			if current.Start <= index && index <= current.End {
				found = i
			}
		}
		window := windows[found]
		windows[found].End = index

		windows = append(windows, qhatWindow{Start: index, End: window.End})
		sort.Slice(windows, func(i, j int) bool {
			return windows[i].Start < windows[j].Start
		})
	}
	return windows, index
}

// createCandidates generates possible change points. If there are no change points then, calculate the required values
// for the whole series. If there are change points, then find the new candidate and split it.
func (d *qhatDetector) createCandidates(series []float64, candidates []qhatChangePoint, index int) []qhatChangePoint {
	if len(candidates) == 0 {
		winQs := d.qHat(series)
		winIndexMax, winMaxQs := d.extractQ(winQs)

		candidates = append(candidates, qhatChangePoint{
			Index:      winIndexMax,
			Q:          winMaxQs,
			Order:      0,
			qhatWindow: qhatWindow{Start: 0, End: len(series)},
		})
	} else {
		found := 0
		for i, current := range candidates {
			if current.Index == index {
				found = i
			}
		}

		start := candidates[found].Start
		end := candidates[found].End

		buffer := series[start:index]

		winQs := d.qHat(buffer)
		winIndexMax, winMaxQs := d.extractQ(winQs)
		candidates[found].Index = winIndexMax + start
		candidates[found].Q = winMaxQs
		candidates[found].Start = start
		candidates[found].End = index

		buffer = series[index:end]
		winQs = d.qHat(buffer)
		winIndexMax, winMaxQs = d.extractQ(winQs)
		candidates = append(candidates, qhatChangePoint{
			Index:      winIndexMax + index,
			Q:          winMaxQs,
			qhatWindow: qhatWindow{Start: index, End: end},
		})

		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].Q < candidates[j].Q
		})
	}
	return candidates
}

func (d *qhatDetector) DetectChanges(series []float64) ([]ChangePoint, error) {
	changePoints := make([]qhatChangePoint, 0, 10)
	length := len(series)
	windows := make([]qhatWindow, 0)
	candidates := make([]qhatChangePoint, 0)

	probability := 0.0
	index := 0

	for probability <= d.pvalue {
		windows, index = d.createWindows(windows, index, length)
		candidates = d.createCandidates(series, candidates, index)

		candidateQ := candidates[len(candidates)-1]
		countAbove := 0.0 // results from permuted test >= candidateQ
		for i := 0; i < d.permutations; i++ {
			if d.shuffleWindows(series, windows, candidateQ.Q) {
				countAbove++
			}
		}

		probability = (1.0 + countAbove) / float64(d.permutations+1)

		// If we are not done, add the change point and setup the next iteration.
		if probability <= d.pvalue {
			changePoint := candidates[len(candidates)-1]
			changePoint.Probability = probability
			changePoints = append(changePoints, changePoint)
			index = changePoint.Index
		}
	}

	sort.Slice(changePoints, func(i, j int) bool {
		return changePoints[i].Order < changePoints[j].Order
	})
	out := make([]ChangePoint, len(changePoints))
	for idx := range changePoints {
		out[idx] = ChangePoint{
			Index: changePoints[idx].Index,
			Info:  d.info,
		}
	}
	return out, nil
}
