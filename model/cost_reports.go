package model

import (
	"time"

	"github.com/evergreen-ci/sink"
	"github.com/pkg/errors"
)

type CostReports struct {
	reports   []CostReport
	env       sink.Environment
	populated bool
}

func (r *CostReports) Setup(e sink.Environment) { r.env = e }
func (r *CostReports) IsNil() bool              { return r.populated }
func (r *CostReports) Size() int                { return len(r.reports) }
func (r *CostReports) Slice() []CostReport      { return r.reports }

func (r *CostReports) Find(start, end time.Time) error {
	conf, session, err := sink.GetSessionWithConfig(r.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	r.populated = false
	err = session.DB(conf.DatabaseName).C(costReportCollection).Find(map[string]interface{}{
		costReportReportKey + "." + costReportMetadataBeginKey: start,
		costReportReportKey + "." + costReportMetadataEndKey:   end,
	}).All(r.reports)

	if err != nil {
		return errors.Wrap(err, "problem finding cost reports within range")
	}
	r.populated = true

	return nil
}

func (r *CostReports) Count() (int, error) {
	conf, session, err := sink.GetSessionWithConfig(r.env)
	if err != nil {
		return -1, errors.WithStack(err)
	}
	defer session.Close()

	n, err := session.DB(conf.DatabaseName).C(costReportCollection).Count()
	if err != nil {
		return -1, errors.Wrap(err, "problem counting number of reports")
	}

	return n, nil
}

type CostReportSummaries struct {
	reports   []CostReportSummary
	env       sink.Environment
	populated bool
}

func (r *CostReportSummaries) Setup(e sink.Environment)   { r.env = e }
func (r *CostReportSummaries) IsNil() bool                { return r.populated }
func (r *CostReportSummaries) Size() int                  { return len(r.reports) }
func (r *CostReportSummaries) Slice() []CostReportSummary { return r.reports }

func (r *CostReports) Find(start, end time.Time) error {
	conf, session, err := sink.GetSessionWithConfig(r.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	r.populated = false
	err = session.DB(conf.DatabaseName).C(costReportSummaryCollection).Find(map[string]interface{}{
		costReportSummaryMetadataKey + "." + costReportMetadataBeginKey: start,
		costReportSummaryMetadataKey + "." + costReportMetadataEndKey:   end,
	}).All(r.reports)

	if err != nil {
		return errors.Wrap(err, "problem finding cost reports within range")
	}
	r.populated = true

	return nil
}

func (r *CostReportSummaries) Count() (int, error) {
	conf, session, err := sink.GetSessionWithConfig(r.env)
	if err != nil {
		return -1, errors.WithStack(err)
	}
	defer session.Close()

	n, err := session.DB(conf.DatabaseName).C(costReportSummaryCollection).Count()
	if err != nil {
		return -1, errors.Wrap(err, "problem counting number of reports")
	}

	return n, nil
}
