package depgraph

import (
	"gonum.org/v1/gonum/graph"
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
