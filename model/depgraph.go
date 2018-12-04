package model

import (
	"fmt"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/depgraph"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
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
	env       cedar.Environment
}

var (
	graphMetadataIDKey       = bsonutil.MustHaveTag(GraphMetadata{}, "BuildID")
	graphMetadataCompleteKey = bsonutil.MustHaveTag(GraphMetadata{}, "Complete")
)

func (g *GraphMetadata) Setup(e cedar.Environment) { g.env = e }
func (g *GraphMetadata) IsNil() bool              { return !g.populated }

func (g *GraphMetadata) Save() error {
	if g.BuildID == "" {
		return errors.New("cannot save document without id")
	}
	conf, session, err := cedar.GetSessionWithConfig(g.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	return errors.WithStack(session.DB(conf.DatabaseName).C(depMetadataCollection).Insert(g))
}

func (g *GraphMetadata) Find() error {
	conf, session, err := cedar.GetSessionWithConfig(g.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	g.populated = false
	err = session.DB(conf.DatabaseName).C(depMetadataCollection).FindId(g.BuildID).One(g)
	if db.ResultsNotFound(err) {
		return errors.Wrapf(err, "could not find document with id '%s'", g.BuildID)
	} else if err != nil {
		return errors.Wrap(err, "problem running graph metadata query")
	}
	g.populated = true

	return nil
}

func (g *GraphMetadata) MarkComplete() error {
	conf, session, err := cedar.GetSessionWithConfig(g.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	err = session.DB(conf.DatabaseName).C(depMetadataCollection).UpdateId(g.BuildID,
		db.Document{
			"$set": db.Document{graphMetadataCompleteKey: true},
		})

	if err != nil {
		return errors.WithStack(err)
	}

	g.Complete = true

	return nil
}

func (g *GraphMetadata) RemoveEdges() error {
	if !g.populated {
		return errors.New("cannot remove edges for an unpopulated graph")
	}

	conf, session, err := cedar.GetSessionWithConfig(g.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	info, err := session.DB(conf.DatabaseName).C(depEdgeCollection).RemoveAll(db.Document{
		graphEdgeGraphKey: g.BuildID,
	})

	grip.Info(message.Fields{
		"operation":   "removing nodes",
		"graph_id":    g.BuildID,
		"change_info": info,
	})

	return errors.WithStack(err)
}

func (g *GraphMetadata) RemoveNodes() error {
	if !g.populated {
		return errors.New("cannot remove edges for an unpopulated graph")
	}

	conf, session, err := cedar.GetSessionWithConfig(g.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	info, err := session.DB(conf.DatabaseName).C(depNodeCollection).RemoveAll(db.Document{
		graphNodeGraphNameKey: g.BuildID,
	})

	grip.Info(message.Fields{
		"operation":   "removing nodes",
		"graph_id":    g.BuildID,
		"change_info": info,
	})
	return errors.WithStack(err)
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
		ID:        fmt.Sprintf("%s.%d", g.BuildID, source.ID()),
		Graph:     g.BuildID,
		Edge:      *source,
		populated: true,
		env:       g.env,
	}
}

func (g *GraphMetadata) edgeQuery(conf *cedar.Configuration, session db.Session) db.Query {
	return session.DB(conf.DatabaseName).C(depEdgeCollection).Find(map[string]interface{}{
		graphEdgeGraphKey: g.BuildID,
	})
}

func (g *GraphMetadata) GetEdges() (db.Iterator, error) {
	conf, s, err := cedar.GetSessionWithConfig(g.env)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	session := db.WrapSession(s)

	return db.NewCombinedIterator(session, g.edgeQuery(conf, session).Iter()), nil
}

func (g *GraphMetadata) AllEdges() ([]GraphEdge, error) {
	conf, s, err := cedar.GetSessionWithConfig(g.env)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	session := db.WrapSession(s)

	defer session.Close()
	out := []GraphEdge{}

	if err := g.edgeQuery(conf, session).All(out); err != nil {
		return nil, errors.WithStack(err)
	}

	return out, nil
}

func (g *GraphMetadata) nodeQuery(conf *cedar.Configuration, session db.Session) db.Query {
	return session.DB(conf.DatabaseName).C(depNodeCollection).Find(map[string]interface{}{
		graphNodeGraphNameKey: g.BuildID,
	})
}

func (g *GraphMetadata) GetNodes() (db.Iterator, error) {
	conf, s, err := cedar.GetSessionWithConfig(g.env)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	session := db.WrapSession(s)

	return db.NewCombinedIterator(session, g.nodeQuery(conf, session).Iter()), nil
}

func (g *GraphMetadata) AllNodes() ([]GraphNode, error) {
	conf, s, err := cedar.GetSessionWithConfig(g.env)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	session := db.WrapSession(s)
	defer session.Close()

	out := []GraphNode{}
	if err = g.nodeQuery(conf, session).All(out); err != nil {
		return nil, errors.WithStack(err)
	}

	return out, nil
}

func (g *GraphMetadata) Resolve() (*depgraph.Graph, error) {
	iter, err := g.GetEdges()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	out := &depgraph.Graph{}
	edge := &GraphEdge{}
	for iter.Next(edge) {
		out.Edges = append(out.Edges, edge.Edge)
	}

	if err = iter.Close(); err != nil {
		return nil, errors.WithStack(err)
	}

	iter, err = g.GetNodes()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	node := &GraphNode{}
	for iter.Next(node) {
		out.Nodes = append(out.Nodes, node.Node)
	}

	if err = iter.Close(); err != nil {
		return nil, errors.WithStack(err)
	}

	return out, nil
}
