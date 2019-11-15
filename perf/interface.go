package perf

// ChangeDetector types calculate change points.
type ChangeDetector interface {
	DetectChanges([]float64) ([]ChangePoint, error)
}

type ChangePoint struct {
	Index int
	Info  AlgorithmInfo
}

type AlgorithmInfo struct {
	Name    string
	Version int
	Options []AlgorithmOption
}

type AlgorithmOption struct {
	Name  string
	Value interface{}
}
