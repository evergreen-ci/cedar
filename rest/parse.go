package rest

import (
	"net/url"
	"time"

	"github.com/evergreen-ci/cedar/util"
	"github.com/pkg/errors"
)

func parseTimeRange(vals url.Values, start, end string) (util.TimeRange, error) {
	tr := util.TimeRange{EndAt: time.Now()}
	startAt := vals.Get(start)
	endAt := vals.Get(end)

	if startAt != "" {
		s, err := time.ParseInLocation(time.RFC3339, startAt, time.UTC)
		if err != nil {
			return util.TimeRange{}, errors.Errorf("problem parsing start time '%s'", startAt)
		}
		tr.StartAt = s
	}

	if endAt != "" {
		e, err := time.ParseInLocation(time.RFC3339, endAt, time.UTC)
		if err != nil {
			return util.TimeRange{}, errors.Errorf("problem parsing end time '%s'", endAt)
		}
		tr.EndAt = e
	}

	return tr, nil
}
