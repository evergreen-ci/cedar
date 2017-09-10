package cost

import (
	"time"

	"github.com/pkg/errors"
)

const layout = "2006-01-02T15:04" //Using reference Mon Jan 2 15:04:05 -0700 MST 2006

type timeRange struct {
	start time.Time
	end   time.Time
}

// getTimes takes in a string of the form "YYYY-MM-DDTHH:MM" as the start
// time for the report, and converts this to time.Time type. If the given string
// is empty, we instead default to using the current time minus the duration.
func getTimes(s string, duration time.Duration) (timeRange, error) {
	var startTime, endTime time.Time
	var err error
	var res timeRange
	if s != "" {
		startTime, err = time.Parse(layout, s)
		if err != nil {
			return res, errors.Wrap(err, "incorrect start format: "+
				"should be YYYY-MM-DDTHH:MM")
		}
		endTime = startTime.Add(duration)
	} else {
		endTime = time.Now()
		startTime = endTime.Add(-duration)
	}
	res.start = startTime
	res.end = endTime

	return res, nil
}
