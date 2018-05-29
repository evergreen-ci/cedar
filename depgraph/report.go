package depgraph

import "github.com/tychoish/tarjan"

type CycleReport struct {
	Cycles [][]string          `json:"cycles"`
	Graph  map[string][]string `json:"graph"`
}

func NewCycleReport(mapping GraphMap) *CycleReport {
	cycles := [][]string{}
	connections := tarjan.Connections(mapping)
	for _, c := range connections {
		if len(c) == 1 {
			continue
		}
		cycles = append(cycles, c)
	}

	return &CycleReport{
		Graph:  mapping,
		Cycles: cycles,
	}
}
