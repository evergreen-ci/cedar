package model

import (
	"fmt"

	"github.com/evergreen-ci/sink/db"
	"github.com/evergreen-ci/sink/db/bsonutil"
	"github.com/pkg/errors"
	"github.com/tychoish/depgraph"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

const (
	depMetadataCollection = "depgraph.graphs"
	depNodeCollection     = "depgraph.nodes"
	depEdgeCollection     = "depgraph.edges"
)

type GraphMetadata struct {
	BuildID string `bson:"_id"`

	populated bool
}

var (
	graphMetadataIDKey = bsonutil.MustHaveTag(GraphMetadata{}, "BuildID")
)

func (g *GraphMetadata) IsNil() bool { return g.populated }

func (g *GraphMetadata) Insert() error { return errors.WithStack(db.Insert(depMetadataCollection, g)) }

func (g *GraphMetadata) Find(id string) error {
	err := db.Query(bson.M{graphMetadataIDKey: id}).FindOne(depMetadataCollection, g)

	g.populated = false
	if err == mgo.ErrNotFound {
		return nil
	}
	g.populated = true

	if err != nil {
		return errors.Wrap(err, "problem running graph metadata query")
	}

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
	}
}

func (g *GraphMetadata) GetEdges() <-chan *GraphEdge {
	out := make(chan *GraphEdge)

	go func() {
		iter := db.Query(bson.M{graphEdgeGraphKey: g.BuildID}).Iter(depEdgeCollection)
		if iter == nil {
			close(out)
			return
		}
		defer iter.Close()

		doc := &GraphEdge{}
		for iter.Next(doc) {
			out <- doc
		}

		close(out)
	}()

	return out
}

func (g *GraphMetadata) AllEdges() ([]*GraphEdge, error) {
	out := []*GraphEdge{}
	err := db.Query(bson.M{graphEdgeGraphKey: g.BuildID}).FindAll(depEdgeCollection, out)

	if err != nil {
		return nil, errors.WithStack(err)
	}

	return out, nil
}

func (g *GraphMetadata) GetNodes() <-chan *GraphNode {
	out := make(chan *GraphEdge)

	go func() {
		iter := db.Query(bson.M{graphNodeGraphNameKey: g.BuildID}).Iter(depNodeCollection)
		if iter == nil {
			close(out)
			return
		}
		defer iter.Close()

		doc := &GraphNode{}
		for iter.Next(doc) {
			out <- doc
		}

		close(out)
	}()

	return out
}

func (g *GraphMetadata) AllNodes() ([]*GraphNode, error) {
	out := []*GraphNode{}
	err := db.Query(bson.M{graphNodeGraphNameKey: g.BuildID}).Iter(depNodeCollection)

	if err != nil {
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
}

func (n *GraphNode) IsNil() bool { return n.populated }

func (n *GraphNode) Insert() error {
	if !n.populated {
		return errors.New("cannot insert non-populated document")
	}

	if n.ID == "" {
		return errors.New("cannot insert document without an ID")
	}

	return errors.WithStack(db.Insert(depNodeCollection, n))
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
}

func (e *GraphEdge) IsNil() bool { return e.populated }

func (e *GraphEdge) Insert() error {
	if !e.populated {
		return errors.New("cannot insert non-populated document")
	}

	if e.ID == "" {
		return errors.New("cannot insert document without an ID")
	}

	return errors.WithStack(db.Insert(depEdgeCollection, e))
}
