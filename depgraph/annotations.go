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
	g.nodeIndex = make(map[int64]Node)
	for _, node := range g.Nodes {
		g.seenID(node.GraphID)
		g.nodes[node.Name] = node
		g.nodeIndex[node.GraphID] = node
	}

	g.edges = make(map[string]Edge)
	g.edgeIndex = make(map[int64]Edge)
	for idx, edge := range g.Edges {
		edge.localID = g.getNextID()

		from := g.nodeIndex[edge.FromNode.GraphID]
		edge.from = &from
		if len(edge.ToNodes) >= 1 {
			to := g.nodeIndex[edge.ToNodes[0].GraphID]
			edge.firstTo = &to
		}

		g.edges[edge.Name()] = edge
		g.edgeIndex[edge.localID] = edge
		g.Edges[idx] = edge
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

// FlattenEdges transforms the edges of the graph so that each edge
// reflects a single connection between two nodes in the graph. As
// generated and imported Edges represent a single source node and
// multiple possible target nodes.
func (g *Graph) FlattenEdges() {
	if g.edgesFlattened {
		return
	}

	edges := []Edge{}
	index := make(map[int64]Edge)
	mapping := make(map[string]Edge)

	for _, edge := range g.Edges {
		if len(edge.ToNodes) <= 1 {
			edges = append(edges, edge)
			index[edge.localID] = edge
			mapping[edge.Name()] = edge
			continue
		}

		for _, target := range edge.ToNodes {
			newEdge := edge.copy()
			newEdge.ToNodes = []NodeRelationship{target}
			to := g.nodeIndex[target.GraphID]
			newEdge.firstTo = &to
		}
	}

	g.Edges = edges
	g.edges = mapping
	g.edgeIndex = index
	g.edgesFlattened = true
}
