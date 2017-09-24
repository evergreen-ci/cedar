package depgraph

import (
	"strings"

	"github.com/mongodb/grip"
)

func typeSliceIs(slice []int, v int) bool {
	for idx := range slice {
		if slice[idx] == v {
			return true
		}
	}

	return false
}

// Filter takes a graph and returns a subset of that graph with only
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
		if typeSliceIs(etint, int(edge.Type())) {
			output.Edges = append(output.Edges, edge)
		}
	}

	for _, node := range g.Nodes {
		if typeSliceIs(ntint, int(node.Type())) {
			output.Nodes = append(output.Nodes, node)
		}
	}

	return &output
}

// Prune takes a graph and removes all nodes with names contain the
// specifying string. This modifies the state of the graph.
func (g *Graph) Prune(matching string) {
	if matching == "" {
		grip.Warning("pruning nodes that match the empty strings is a noop")
		return
	}

	newNodes := []Node{}
	for _, node := range g.Nodes {
		if strings.Contains(node.Name, matching) {
			continue
		}

		newNodes = append(newNodes, node)
	}
	grip.Infof("pruning %d nodes", len(g.Nodes)-len(newNodes))
	g.Nodes = newNodes

	newEdges := []Edge{}
	for _, edge := range g.Edges {
		if strings.Contains(edge.FromNode.Name, matching) {
			continue
		}

		nodes := []NodeRelationship{}
		for _, node := range edge.ToNodes {
			if strings.Contains(node.Name, matching) {
				continue
			}

			nodes = append(nodes, node)
		}
		edge.ToNodes = nodes

		newEdges = append(newEdges, edge)
	}
	grip.Infof("pruning %d edges", len(g.Edges)-len(newEdges))
	g.Edges = newEdges
	g.refresh()
}
