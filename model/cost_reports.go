package model

import (
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
	"github.com/pkg/errors"
)

type CostReports struct {
	reports   []CostReport
	env       cedar.Environment
	populated bool
}

func (r *CostReports) Setup(e cedar.Environment) { r.env = e }
func (r *CostReports) IsNil() bool               { return !r.populated }
func (r *CostReports) Size() int                 { return len(r.reports) }
func (r *CostReports) Slice() []CostReport       { return r.reports }

func (r *CostReports) Find(start, end time.Time) error {
	session, query, err := r.rangeQuery(start, end)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	r.populated = false
	if err = query.All(&r.reports); err != nil {
		return errors.Wrap(err, "problem finding cost reports within range")
	}
	r.populated = true

	return nil
}

func (r *CostReports) Iterator(start, end time.Time) (db.Iterator, error) {
	session, query, err := r.rangeQuery(start, end)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return db.NewCombinedIterator(session, query.Iter()), nil

}

func (r *CostReports) rangeQuery(start, end time.Time) (db.Session, db.Query, error) {
	conf, session, err := cedar.GetSessionWithConfig(r.env)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	query := session.DB(conf.DatabaseName).C(costReportCollection).Find(db.Document{
		bsonutil.GetDottedKeyName(costReportReportKey, costReportMetadataRangeKey, "start"): db.Document{"$gte": start},
		bsonutil.GetDottedKeyName(costReportReportKey, costReportMetadataRangeKey, "end"):   db.Document{"$lt": end},
	})

	return session, query, nil
}

func (r *CostReports) Count() (int, error) {
	conf, session, err := cedar.GetSessionWithConfig(r.env)
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
	env       cedar.Environment
	populated bool
}

func (r *CostReportSummaries) Setup(e cedar.Environment)  { r.env = e }
func (r *CostReportSummaries) IsNil() bool                { return !r.populated }
func (r *CostReportSummaries) Size() int                  { return len(r.reports) }
func (r *CostReportSummaries) Slice() []CostReportSummary { return r.reports }

func (r *CostReportSummaries) Find(start, end time.Time) error {
	session, query, err := r.rangeQuery(start, end)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	r.populated = false
	if err = query.All(r.reports); err != nil {
		return errors.Wrap(err, "problem finding cost reports within range")
	}
	r.populated = true

	return nil
}

func (r *CostReportSummaries) Iterator(start, end time.Time) (db.Iterator, error) {
	session, query, err := r.rangeQuery(start, end)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return db.NewCombinedIterator(session, query.Iter()), nil
}

func (r *CostReportSummaries) rangeQuery(start, end time.Time) (db.Session, db.Query, error) {
	conf, session, err := cedar.GetSessionWithConfig(r.env)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	query := session.DB(conf.DatabaseName).C(costReportSummaryCollection).Find(map[string]interface{}{
		bsonutil.GetDottedKeyName(costReportSummaryMetadataKey, costReportMetadataRangeKey, "start"): start,
		bsonutil.GetDottedKeyName(costReportSummaryMetadataKey, costReportMetadataRangeKey, "end"):   end,
	})

	return session, query, nil
}

func (r *CostReportSummaries) Count() (int, error) {
	conf, session, err := cedar.GetSessionWithConfig(r.env)
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
