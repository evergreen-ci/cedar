package model

import (
	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/bsonutil"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"github.com/tychoish/anser/db"
	"github.com/tychoish/anser/model"
)

const costReportCollection = "buildCostReports"

// CostReport provides the structure for the report that will be returned for
// the build cost reporting tool.
type CostReport struct {
	ID        string             `bson:"_id" json:"-" yaml:"-"`
	Report    CostReportMetadata `bson:"report" json:"report" yaml:"report"`
	Evergreen EvergreenCost      `bson:"evergreen" json:"evergreen" yaml:"evergreen"`
	Providers []CloudProvider    `bson:"providers" json:"providers" yaml:"providers"`

	env       sink.Environment
	populated bool
}

var (
	costReportReportKey    = bsonutil.MustHaveTag(CostReport{}, "Report")
	costReportEvergreenKey = bsonutil.MustHaveTag(CostReport{}, "Evergreen")
	costReportProvidersKey = bsonutil.MustHaveTag(CostReport{}, "Providers")
)

func (r *CostReport) Setup(e sink.Environment) { r.env = e }
func (r *CostReport) IsNil() bool              { return r.populated }
func (r *CostReport) FindID(id string) error {
	conf, session, err := sink.GetSessionWithConfig(r.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	r.populated = false

	err = session.DB(conf.DatabaseName).C(costReportCollection).FindId(id).One(r)
	if db.ResultsNotFound(err) {
		return errors.Errorf("could not find cost reporting document %s in the database", id)
	} else if err != nil {
		errors.Wrap(err, "problem finding cost config document")
	}
	r.populated = true

	return nil
}

func (r *CostReport) Save() error {
	// TOOD call some kind of validation routine to avoid saving junk data
	conf, session, err := sink.GetSessionWithConfig(r.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	changeInfo, err := session.DB(conf.DatabaseName).C(costReportCollection).UpsertId(r.ID, r)
	grip.Debug(message.Fields{
		"ns":          model.Namespace{DB: conf.DatabaseName, Collection: costReportCollection},
		"id":          r.ID,
		"operation":   "save build cost report",
		"change-info": changeInfo,
	})
	if db.ResultsNotFound(err) {
		return errors.New("could not find cost reporting document in the database")
	}

	return errors.Wrap(err, "problem saving cost reporting configuration")
}
