package rest

import (
	"time"

	"github.com/evergreen-ci/cedar/model"
	"github.com/pkg/errors"
)

func parseTimeRange(format, start, end string) (model.TimeRange, error) {
	tr := model.TimeRange{EndAt: time.Now()}

	if start != "" {
		s, err := time.ParseInLocation(format, start, time.UTC)
		if err != nil {
			return model.TimeRange{}, errors.Errorf("problem parsing start time '%s'", start)
		}
		tr.StartAt = s
	}

	if end != "" {
		e, err := time.ParseInLocation(format, end, time.UTC)
		if err != nil {
			return model.TimeRange{}, errors.Errorf("problem parsing end time '%s'", end)
		}
		tr.EndAt = e
	}

	return tr, nil
}
