package depgraph

import (
	"strings"
)

func typeSliceIs(slice []int, v int) bool {
	for idx := range slice {
		if slice[idx] == v {
			return true
		}
	}

	return false
}

// Fliter takes a graph and returns a subset of that graph with only
// the specified node and edge types.
func (g *Graph) Filter(et []EdgeType, nt []NodeType) *Graph {
	output := Graph{}

	etint := make([]int, len(et))
	ntint := make([]int, len(nt))
	for idx := range et {
		etint[idx] = int(et[idx])
	}
	for idx := range nt {
		ntint[idx] = int(nt[idx])
	}

	for _, edge := range g.Edges {
		if typeSliceIs(etint, int(edge.Type)) {
			output.Edges = append(output.Edges, edge)
		}
	}

	for _, node := range g.Nodes {
		if typeSliceIs(ntint, int(node.Relationships.Type)) {
			output.Nodes = append(output.Nodes, node)
		}
	}

	return &output
}

type GraphMap map[string][]string

// Graph renders the edges of a graph into
func (g *Graph) Mapping(stripPrefix string) GraphMap {
	out := make(map[string][]string)

	for _, edge := range g.Edges {
		if len(edge.ToNodes) == 0 {
			continue
		}

		targets := make([]string, len(edge.ToNodes))
		for idx, toEdge := range edge.ToNodes {
			targets[idx] = strings.TrimLeft(toEdge.Name, stripPrefix)
		}

		out[strings.TrimLeft(edge.FromNode.Name, stripPrefix)] = targets
	}

	return GraphMap(out)
}
