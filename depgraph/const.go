package depgraph

//go:generate stringer -type=EdgeType
//go:generate stringer -type=NodeType

// EdgeType reflects the nature of the relationship between two points
// on the graph.
type EdgeType int

const (
	_ EdgeType = iota
	LibraryToLibrary
	LibraryToFile
	FileToLibrary
	FileToFile
	FileToSymbol
	LibraryToSymbol
	ImplicitLibraryToLibrary
	ArtifactToLibrary
)

// NodeType reflects the type of the node.
type NodeType int

const (
	_ NodeType = iota
	Library
	Symbol
	File
	Artifact
)
