package model

import (
	"fmt"

	"github.com/evergreen-ci/sink"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
	"github.com/pkg/errors"
	"github.com/tychoish/depgraph"
)

const (
	depMetadataCollection = "depgraph.graphs"
	depNodeCollection     = "depgraph.nodes"
	depEdgeCollection     = "depgraph.edges"
)

type GraphMetadata struct {
	BuildID   string `bson:"_id"`
	Complete  bool   `bson:"complete"`
	populated bool
	env       sink.Environment
}

var (
	graphMetadataIDKey       = bsonutil.MustHaveTag(GraphMetadata{}, "BuildID")
	graphMetadataCompleteKey = bsonutil.MustHaveTag(GraphMetadata{}, "Complete")
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

func (g *GraphMetadata) MarkComplete() error {
	conf, session, err := sink.GetSessionWithConfig(g.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	err = session.DB(conf.DatabaseName).C(depMetadataCollection).UpdateId(g.BuildID,
		db.Document{
			"$set": db.Document{graphMetadataCompleteKey: true},
		})

	return errors.WithStack(err)
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
		return errors.Wrapf(err, "could not find document with id '%s'", id)
	} else if err != nil {
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
