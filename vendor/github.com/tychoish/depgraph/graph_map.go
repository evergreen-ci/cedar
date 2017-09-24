package depgraph

import (
	"strings"

	"github.com/awalterschulze/gographviz"
	"github.com/mongodb/grip"
)

// GraphMap is an alias for map[string][]string which is a convenient
// format for representating DAGs, and is the basis for other output
// formats.
type GraphMap map[string][]string

// Graph renders the edges of a graph into a GraphMap.
func (g *Graph) Mapping(stripPrefix string) GraphMap {
	out := GraphMap{}

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

	grip.Infof("rendered graph output mapping with %d nodes", len(out))
	return out
}

func cleanNameForDot(name string) string {
	return strings.Join([]string{`"`, `"`}, strings.Replace(name, `"`, `\"`, -1))
}

// Dot transform a graph mapping.
func (report GraphMap) Dot() string {
	const name = "libdeps"
	dot := gographviz.NewGraph()

	dot.SetName(name)
	dot.SetDir(true)

	for node, edges := range report {
		node = cleanNameForDot(node)
		dot.AddNode(name, node, nil)

		for _, edge := range edges {
			dot.AddEdge(node, cleanNameForDot(edge), true, nil)
		}
	}

	grip.Infof("rendering dot file with %d nodes and %d edges to graph", len(report), len(dot.Edges.Edges))
	return dot.String()
}
