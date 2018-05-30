package depgraph

import (
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
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
		dag.AddNode(n)
	}

	for _, e := range g.Edges {
		if e.from == nil {
			grip.Warning(message.Fields{
				"message": "edge missing incoming node",
				"name":    e.Name(),
			})
			continue
		}

		if e.firstTo == nil {
			grip.Warning(message.Fields{
				"message": "edge missing outgoing node",
				"name":    e.Name(),
			})
			continue
		}

		dag.SetEdge(e)
	}

	return dag
}

func (g *Graph) AllBetween(from, to string) ([][]string, error) {

	fromNode, ok := g.nodes[from]
	if !ok {
		return nil, errors.Errorf("could not find from node [%s]", from)
	}

	toNode, ok := g.nodes[to]
	if !ok {
		return nil, errors.Errorf("could not find to node, [%s]", to)
	}

	paths, ok := path.JohnsonAllPaths(g.Directed())
	if !ok {
		return nil, errors.New("could not resolve paths from graph")
	}

	all, _ := paths.AllBetween(fromNode.ID(), toNode.ID())
	if len(all) == 0 {
		return nil, errors.Errorf("found no path between nodes, [%s => %s]", from, to)
	}

	out := [][]string{}
	for _, group := range all {
		gr := []string{}

		for _, n := range group {
			if n == nil {
				grip.Warning("found nil node")
				continue
			}
			gr = append(gr, g.nodeIndex[n.ID()].Name)
		}
		out = append(out, gr)
	}

	return out, nil
}
