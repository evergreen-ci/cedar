package model

import (
	"github.com/pkg/errors"

	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/anser/db"
)

type DependencyGraphs struct {
	graphs []GraphMetadata

	populated bool
	env       cedar.Environment
}

func (g *DependencyGraphs) Setup(e cedar.Environment) { g.env = e }
func (g *DependencyGraphs) IsNil() bool               { return !g.populated }
func (g *DependencyGraphs) Size() int                 { return len(g.graphs) }
func (g *DependencyGraphs) Slice() []GraphMetadata    { return g.graphs }
func (g *DependencyGraphs) FindIncomplete() error {
	conf, session, err := cedar.GetSessionWithConfig(g.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	query := session.DB(conf.DatabaseName).C(depMetadataCollection).Find(db.Document{
		graphMetadataCompleteKey: false,
	})

	g.populated = false

	err = query.All(g.graphs)

	if db.ResultsNotFound(err) {
		return nil
	}

	g.populated = true

	return errors.WithStack(err)
}
