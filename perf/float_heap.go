package perf

import "sort"

// An FloatHeap is a min-sorted-list of floats.
type sortedList []float64

// Clear the list.
func (s *sortedList) Clear() {
	*s = (*s)[:0]
}

// Insert the list of floats maintaining the sort order.
func (s *sortedList) Insert(floats ...float64) {
	length := len(*s)
	for _, f := range floats {
		// Assume f is appended.
		*s = append(*s, f)
		if length != 0 && f < (*s)[length-1] {
			// Deal with the case where we are inserting before the end.
			index := sort.SearchFloat64s(*s, f)
			copy((*s)[index+1:], (*s)[index:])
			(*s)[index] = f
		}
		length += 1
	}
	// note: the lines above are about 3 times faster than inserting and then sorting
	//*s = append(*s, floats...)
	//sort.Sort((*s))
}

// Remove f, maintaining the sort order.
func (s *sortedList) Remove(f float64) {
	index := sort.Search(len(*s), func(i int) bool { return (*s)[i] > f })
	length := len(*s)
	if index == 0 {
		if (*s)[index] == f {
			*s = (*s)[1:]
		}
	} else if index == length {
		*s = (*s)[0 : length-1]
	} else {
		copy((*s)[index-1:], (*s)[index:])
		*s = (*s)[:length-1]
	}
}

// Calculate the median, assuming the list is sorted.
func (s *sortedList) Median() float64 {
	length := len(*s)
	center := int(length / 2)
	if length%2 != 0 {
		return (*s)[center]
	}
	median := ((*s)[center] + (*s)[center-1]) / 2.0
	return median
}
