package depgraph

func (g *Graph) Annotate() {
	// do things here to
	if !g.mapsPopulated {
		g.refresh()
	}

	g.addAllImplicitLibraryDependencyEdges()
}

func (g *Graph) refresh() {
	g.edges = make(map[string]Edge)
	for _, edge := range g.Edges {
		g.edges[edge.ID()] = edge
	}

	g.nodes = make(map[string]Node)
	for _, node := range g.Nodes {
		g.nodes[node.Name] = node
	}
	g.mapsPopulated = true
}
