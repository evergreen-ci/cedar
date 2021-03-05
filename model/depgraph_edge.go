package model

import (
	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/depgraph"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
	"github.com/pkg/errors"
)

var (
	graphEdgeIDKey       = bsonutil.MustHaveTag(GraphEdge{}, "ID")
	graphEdgeGraphKey    = bsonutil.MustHaveTag(GraphEdge{}, "Graph")
	graphEdgeTypeKey     = bsonutil.MustHaveTag(GraphEdge{}, "Type")
	graphEdgeFromNodeKey = bsonutil.MustHaveTag(GraphEdge{}, "FromNode")
	graphEdgeToNodeKey   = bsonutil.MustHaveTag(GraphEdge{}, "ToNodes")

	graphRelationshipGraphIDKey = bsonutil.MustHaveTag(depgraph.NodeRelationship{}, "GraphID")
	graphRelationshipNameKey    = bsonutil.MustHaveTag(depgraph.NodeRelationship{}, "Name")
)

type GraphEdge struct {
	ID            string `bson:"_id"`
	Graph         string `bson:"graph"`
	depgraph.Edge `bson:"inline"`

	populated bool
	env       cedar.Environment
}

func (e *GraphEdge) Setup(env cedar.Environment) { e.env = env }
func (e *GraphEdge) IsNil() bool                 { return !e.populated }
func (e *GraphEdge) Save() error {
	if !e.populated {
		return errors.New("cannot insert non-populated document")
	}

	if e.ID == "" {
		return errors.New("cannot insert document without an ID")
	}

	conf, session, err := cedar.GetSessionWithConfig(e.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	return errors.WithStack(session.DB(conf.DatabaseName).C(depEdgeCollection).Insert(e))
}

func (e *GraphEdge) Find() error {
	conf, session, err := cedar.GetSessionWithConfig(e.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()
	e.populated = false
	err = session.DB(conf.DatabaseName).C(depEdgeCollection).FindId(e.ID).One(e)
	if db.ResultsNotFound(err) {
		return errors.New("could not find matching edge")
	} else if err != nil {
		return errors.Wrap(err, "problem finding graph edge")
	}
	e.populated = true

	return nil
}
