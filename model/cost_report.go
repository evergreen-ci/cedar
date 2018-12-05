package model

import (
	"encoding/json"

	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/anser/model"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

const costReportCollection = "spendingreports"

// CostReport provides the structure for the report that will be returned for
// the build cost reporting tool.
type CostReport struct {
	ID        string             `bson:"_id" json:"-" yaml:"-"`
	Report    CostReportMetadata `bson:"report" json:"report" yaml:"report"`
	Evergreen EvergreenCost      `bson:"evergreen" json:"evergreen" yaml:"evergreen"`
	Providers []CloudProvider    `bson:"providers" json:"providers" yaml:"providers"`

	env       cedar.Environment
	populated bool
	providers map[string]*CloudProvider
}

func (r *CostReport) refresh() {
	r.providers = make(map[string]*CloudProvider)
	for _, p := range r.Providers {
		p.refresh(r.Report.Range)
		r.providers[p.Name] = &p
	}
	r.Evergreen.refresh()
}

var (
	costReportReportKey    = bsonutil.MustHaveTag(CostReport{}, "Report")
	costReportEvergreenKey = bsonutil.MustHaveTag(CostReport{}, "Evergreen")
	costReportProvidersKey = bsonutil.MustHaveTag(CostReport{}, "Providers")
)

func (r *CostReport) String() string {
	jsonReport, _ := json.MarshalIndent(r, "", "    ") // pretty print
	return string(jsonReport)
}

func (r *CostReport) Setup(e cedar.Environment) { r.env = e; r.refresh() }
func (r *CostReport) IsNil() bool              { return !r.populated }
func (r *CostReport) Find() error {
	conf, session, err := cedar.GetSessionWithConfig(r.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	r.populated = false

	err = session.DB(conf.DatabaseName).C(costReportCollection).FindId(r.ID).One(r)
	if db.ResultsNotFound(err) {
		return errors.Errorf("could not find cost reporting document %s in the database", r.ID)
	} else if err != nil {
		return errors.Wrap(err, "problem finding cost config document")
	}
	r.populated = true

	r.refresh()
	return nil
}

func (r *CostReport) Save() error {
	if r.ID == "" {
		return errors.New("cannot populate a cost report document without specifying an ID")
	}

	// TOOD call some kind of validation routine to avoid saving junk data
	conf, session, err := cedar.GetSessionWithConfig(r.env)
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

	return errors.Wrap(err, "problem saving cost reporting configuration")
}
