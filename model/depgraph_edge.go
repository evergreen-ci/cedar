package model

import (
	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/depgraph"
	"github.com/mongodb/anser/bsonutil"
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
	env       sink.Environment
}

func (e *GraphEdge) Setup(env sink.Environment) { e.env = env }
func (e *GraphEdge) IsNil() bool                { return e.populated }
func (e *GraphEdge) Insert() error {
	if !e.populated {
		return errors.New("cannot insert non-populated document")
	}

	if e.ID == "" {
		return errors.New("cannot insert document without an ID")
	}

	conf, session, err := sink.GetSessionWithConfig(e.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	return errors.WithStack(session.DB(conf.DatabaseName).C(depEdgeCollection).Insert(e))
}
