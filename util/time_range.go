package util

import (
	"time"

	"github.com/mongodb/anser/bsonutil"
)

type TimeRange struct {
	StartAt time.Time `bson:"start" json:"start" yaml:"start"`
	EndAt   time.Time `bson:"end" json:"end" yaml:"end"`
}

var (
	timeRangeStartKey = bsonutil.MustHaveTag(TimeRange{}, "StartAt")
	timeRangeEndKey   = bsonutil.MustHaveTag(TimeRange{}, "EndAt")
)

func (t TimeRange) Duration() time.Duration { return t.EndAt.Sub(t.StartAt) }
func (t TimeRange) IsZero() bool            { return t.EndAt.IsZero() && t.StartAt.IsZero() }
func (t TimeRange) IsValid() bool           { return t.Duration() >= 0 }

// Check returns true if the given time is within the TimeRange (inclusive) and
// false otherwise.
func (t TimeRange) Check(ts time.Time) bool {
	if (ts.After(t.StartAt) || ts.Equal(t.StartAt)) &&
		(ts.Before(t.EndAt) || ts.Equal(t.EndAt)) {
		return true
	}
	return false
}

// GetTimeRange builds a time range structure. If startAt is the zero
// time, then end defaults to the current time and the start time is
// determined by the duration. Otherwise the end time is determined
// using the duration.
func GetTimeRange(startAt time.Time, duration time.Duration) TimeRange {
	var endTime time.Time

	if startAt.IsZero() {
		endTime = time.Now()
		startAt = endTime.Add(-duration)
	} else {
		endTime = startAt.Add(duration)
	}

	return TimeRange{
		StartAt: startAt,
		EndAt:   endTime,
	}
}
