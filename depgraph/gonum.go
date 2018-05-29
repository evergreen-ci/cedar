package depgraph

import (
	"errors"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/path"
	"gonum.org/v1/gonum/graph/simple"
)

// From provides a pointer to the "from" edge. Implemented to
// support gonum/graph algorithms.
func (e Edge) From() graph.Node { return e.from }

// To returns a pointer to the id of the first target node
// defined in the edge. It ignores additional targets in the current
// implemenation.
func (e Edge) To() graph.Node { return e.firstTo }

func (g *Graph) Directed() graph.Directed {
	dag := simple.NewDirectedGraph()

	for _, n := range g.Nodes {
		dag.AddNode(&n)
	}

	for _, e := range g.Edges {
		dag.SetEdge(&e)
	}

	return dag
}

func (g *Graph) AllBetween(from, to string) ([][]*Node, error) {

	fromNode, ok := g.nodes[from]
	if !ok {
		return nil, errors.New("could not find from node")
	}
	toNode, ok := g.nodes[to]
	if !ok {
		return nil, errors.New("could not find to node")
	}
	paths, ok := path.JohnsonAllPaths(g.Directed())
	if !ok {
		return nil, errors.New("could not find all paths")
	}

	all, _ := paths.AllBetween(fromNode.ID(), toNode.ID())
	if len(all) == 0 {
		return nil, errors.New("found no path between nodes")
	}

	out := [][]*Node{}
	for _, group := range all {
		gr := []*Node{}

		for _, n := range group {
			node := g.nodeIndex[n.ID()]
			gr = append(gr, &node)
		}
		out = append(out, gr)
	}

	return out, nil
}
