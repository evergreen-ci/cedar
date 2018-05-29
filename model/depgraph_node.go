package model

import (
	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/depgraph"
	"github.com/mongodb/anser/bsonutil"
	"github.com/pkg/errors"
)

var (
	graphNodeIDKey                = bsonutil.MustHaveTag(GraphNode{}, "ID")
	graphNodeGraphNameKey         = bsonutil.MustHaveTag(GraphNode{}, "GraphName")
	graphNodeNameKey              = bsonutil.MustHaveTag(GraphNode{}, "Name")
	graphNodeGraphIDKey           = bsonutil.MustHaveTag(GraphNode{}, "GraphID")
	graphNodeRelationshipsKey     = bsonutil.MustHaveTag(GraphNode{}, "Relationships")
	graphNodeRelationshipsTypeKey = bsonutil.MustHaveTag(GraphNode{}.Relationships, "Type")
)

type GraphNode struct {
	ID            string `bson:"_id"`
	GraphName     string `bson:"graph"`
	depgraph.Node `bson:"inline"`

	populated bool
	env       sink.Environment
}

func (n *GraphNode) Setup(e sink.Environment) { n.env = e }
func (n *GraphNode) IsNil() bool              { return n.populated }
func (n *GraphNode) Insert() error {
	if !n.populated {
		return errors.New("cannot insert non-populated document")
	}

	if n.ID == "" {
		return errors.New("cannot insert document without an ID")
	}

	conf, session, err := sink.GetSessionWithConfig(n.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	return errors.WithStack(session.DB(conf.DatabaseName).C(depNodeCollection).Insert(n))
}
