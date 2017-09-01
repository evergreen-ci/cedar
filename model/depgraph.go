package model

import (
	"fmt"

	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/bsonutil"
	"github.com/pkg/errors"
	"github.com/tychoish/anser/db"
	"github.com/tychoish/depgraph"
)

const (
	depMetadataCollection = "depgraph.graphs"
	depNodeCollection     = "depgraph.nodes"
	depEdgeCollection     = "depgraph.edges"
)

type GraphMetadata struct {
	BuildID   string `bson:"_id"`
	populated bool
	env       sink.Environment
}

var (
	graphMetadataIDKey = bsonutil.MustHaveTag(GraphMetadata{}, "BuildID")
)

func (g *GraphMetadata) Setup(e sink.Environment) { g.env = e }
func (g *GraphMetadata) IsNil() bool              { return g.populated }

func (g *GraphMetadata) Insert() error {
	conf, session, err := sink.GetSessionWithConfig(g.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	return errors.WithStack(session.DB(conf.DatabaseName).C(depMetadataCollection).Insert(g))
}

func (g *GraphMetadata) Find(id string) error {
	conf, session, err := sink.GetSessionWithConfig(g.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	g.populated = false
	err = session.DB(conf.DatabaseName).C(depMetadataCollection).FindId(id).One(g)
	if db.ResultsNotFound(err) {
		return nil
	}

	if err != nil {
		return errors.Wrap(err, "problem running graph metadata query")
	}

	g.populated = true

	return nil
}

func (g *GraphMetadata) MakeNode(source *depgraph.Node) *GraphNode {
	if source == nil {
		return nil
	}

	return &GraphNode{
		ID:        fmt.Sprintf("%s.%d", g.BuildID, source.GraphID),
		GraphName: g.BuildID,
		Node:      *source,
		populated: true,
		env:       g.env,
	}
}

func (g *GraphMetadata) MakeEdge(source *depgraph.Edge) *GraphEdge {
	if source == nil {
		return nil
	}

	return &GraphEdge{
		ID:        fmt.Sprintf("%s.%s.%d", g.BuildID, source.Type, source.FromNode.GraphID),
		Graph:     g.BuildID,
		Edge:      *source,
		populated: true,
		env:       g.env,
	}
}

func (g *GraphMetadata) edgeQuery(conf *sink.Configuration, session db.Session) db.Query {
	return session.DB(conf.DatabaseName).C(depEdgeCollection).Find(map[string]interface{}{
		graphEdgeGraphKey: g.BuildID,
	})
}

func (g *GraphMetadata) GetEdges() (db.Iterator, error) {
	conf, session, err := sink.GetSessionWithConfig(g.env)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return db.NewCombinedIterator(session, g.edgeQuery(conf, session).Iter()), nil
}

func (g *GraphMetadata) AllEdges() ([]*GraphEdge, error) {
	conf, session, err := sink.GetSessionWithConfig(g.env)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer session.Close()
	out := []*GraphEdge{}

	if err := g.edgeQuery(conf, session).All(out); err != nil {
		return nil, errors.WithStack(err)
	}

	return out, nil
}

func (g *GraphMetadata) nodeQuery(conf *sink.Configuration, session db.Session) db.Query {
	return session.DB(conf.DatabaseName).C(depNodeCollection).Find(map[string]interface{}{
		graphNodeGraphNameKey: g.BuildID,
	})
}

func (g *GraphMetadata) GetNodes() (db.Iterator, error) {
	conf, session, err := sink.GetSessionWithConfig(g.env)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return db.NewCombinedIterator(session, g.nodeQuery(conf, session).Iter()), nil
}

func (g *GraphMetadata) AllNodes() ([]*GraphNode, error) {
	conf, session, err := sink.GetSessionWithConfig(g.env)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer session.Close()

	out := []*GraphNode{}
	if err = g.nodeQuery(conf, session).All(out); err != nil {
		return nil, errors.WithStack(err)
	}

	return out, nil
}

var (
	graphNodeIDKey                    = bsonutil.MustHaveTag(GraphNode{}, "ID")
	graphNodeGraphNameKey             = bsonutil.MustHaveTag(GraphNode{}, "GraphName")
	graphNodeNameKey                  = bsonutil.MustHaveTag(GraphNode{}, "Name")
	graphNodeGraphIDKey               = bsonutil.MustHaveTag(GraphNode{}, "GraphID")
	graphNodeRelationshipsKey         = bsonutil.MustHaveTag(GraphNode{}, "Relationships")
	graphNodeRelationshipsDepLibsKey  = bsonutil.MustHaveTag(GraphNode{}.Relationships, "DependentLibraries")
	graphNodeRelationshipsLibKey      = bsonutil.MustHaveTag(GraphNode{}.Relationships, "Libraries")
	graphNodeRelationshipsFilesKey    = bsonutil.MustHaveTag(GraphNode{}.Relationships, "Files")
	graphNodeRelationshipsDepFilesKey = bsonutil.MustHaveTag(GraphNode{}.Relationships, "DependentFiles")
	graphNodeRelationshipsTypeKey     = bsonutil.MustHaveTag(GraphNode{}.Relationships, "Type")
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
