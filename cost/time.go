package cost

import (
	"time"
)

const layout = "2006-01-02T15:04" //Using reference Mon Jan 2 15:04:05 -0700 MST 2006

type timeRange struct {
	start time.Time
	end   time.Time
}

// getTimes takes in a string of the form "YYYY-MM-DDTHH:MM" as the start
// time for the report, and converts this to time.Time type. If you
// specify a zero time for start time, we instead default to using the
// current time minus the duration.
func getTimes(startTime time.Time, duration time.Duration) timeRange {
	var (
		endTime time.Time
		res     timeRange
	)

	if startTime.IsZero() {
		endTime = time.Now()
		startTime = endTime.Add(-duration)
	} else {
		endTime = startTime.Add(duration)
	}

	res.start = startTime
	res.end = endTime

	return res
}
