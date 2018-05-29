package depgraph

import (
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
)

func (g *Graph) seenID(id int64) {
	if id <= g.nextID {
		g.nextID = id + 1
	}
}

func (g *Graph) getNextID() int64 {
	g.nextID++
	return g.nextID - 1
}

func (g *Graph) Annotate() {
	// do things here to
	if !g.mapsPopulated {
		g.refresh()
	}

	g.addAllImplicitLibraryDependencyEdges()
}

func (g *Graph) refresh() {
	g.nodes = make(map[string]Node)
	for _, node := range g.Nodes {
		g.seenID(node.GraphID)
		g.nodes[node.Name] = node
		g.nodeIndex[node.GraphID] = node
		if node.GraphID == 0 {

		}
	}

	g.edges = make(map[string]Edge)
	for _, edge := range g.Edges {
		edge.localID = g.nextID
		g.nextID++

		g.edges[edge.Name()] = edge
		g.edgeIndex[edge.localID] = edge

	}

	g.mapsPopulated = true

	// alerting for error detecting
	grip.CriticalWhen(len(g.nodes) != len(g.nodeIndex),
		message.Fields{
			"message":    "graph indexing error",
			"nextID":     g.nextID,
			"node_names": len(g.nodes),
			"node_ids":   len(g.nodeIndex),
			"build_id":   g.BuildID,
		})
	grip.CriticalWhen(len(g.edges) != len(g.edgeIndex),
		message.Fields{
			"message":    "graph indexing error",
			"nextID":     g.nextID,
			"edge_names": len(g.edges),
			"edge_ids":   len(g.edgeIndex),
			"build_id":   g.BuildID,
		})
}
