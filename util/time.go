package util

import (
	"time"
)

// UnixMilli returns t as a Unix time, the number of nanoseconds elapsed since
// January 1, 1970 UTC. The result is undefined if the Unix time in nanoseconds
// in cannot be represented by an int64 (a date before the year 1678 or after
// 2262). Note that this means the result of calling UnixMilli on the zero Time
// on the zero Time is undefined. The result does not depend on the location
// associated with t.
func UnixMilli(t time.Time) int64 {
	return t.UnixNano() / 1e6
}

// ParseRoundPartOfDay produces a time value with the hour value
// rounded down to the most recent interval.
func RoundPartOfDay(num int) time.Time { return findPartHour(time.Now(), num) }

// ParseRoundPartOfHour produces a time value with the minute value
// rounded down to the most recent interval.
func RoundPartOfHour(num int) time.Time { return findPartMin(time.Now(), num) }

// ParseRoundPartOfMinute produces a time value with the second value
// rounded down to the most recent interval.
func RoundPartOfMinute(num int) time.Time { return findPartSec(time.Now(), num) }

// this implements the logic of RoundPartOfDay, but takes time as an
// argument for testability.
func findPartHour(now time.Time, num int) time.Time {
	var hour int

	if num > now.Hour() || num > 12 || num <= 0 {
		hour = 0
	} else {
		hour = now.Hour() - (now.Hour() % num)
	}

	return time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, time.UTC)
}

// this implements the logic of RoundPartOfHour, but takes time as an
// argument for testability.
func findPartMin(now time.Time, num int) time.Time {
	var min int

	if num > now.Minute() || num > 30 || num <= 0 {
		min = 0
	} else {
		min = now.Minute() - (now.Minute() % num)
	}

	return time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), min, 0, 0, time.UTC)
}

// this implements the logic of RoundPartOfMinute, but takes time as an
// argument for testability.
func findPartSec(now time.Time, num int) time.Time {
	var sec int

	if num > now.Second() || num > 30 || num <= 0 {
		sec = 0
	} else {
		sec = now.Second() - (now.Second() % num)
	}

	return time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), sec, 0, time.UTC)

}
