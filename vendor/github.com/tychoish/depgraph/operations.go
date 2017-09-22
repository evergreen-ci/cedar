package depgraph

// Fliter takes a graph and returns a subset of that graph with only
// the specified node and edge types.
func (g *Graph) Filter(et EdgeType, nt NodeType) *Graph {
	output := Graph{}

	for _, edge := range g.Edges {
		if edge.Type == et {
			output.Edges = append(output.Edges, edge)
		}
	}

	for _, node := range g.Nodes {
		if node.Relationships.Type == nt {
			output.Nodes = append(output.Nodes, node)
		}
	}

	return &output
}
