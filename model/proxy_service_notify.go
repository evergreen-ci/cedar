package model

// PerformanceTestResultID is the ID to uniquely identify a new performance result available at test level.
type PerformanceTestResultID struct {
	Version   string           `json:"version"`
	TaskID    string           `json:"task_id"`
	Project   string           `json:"project"`
	Variant   string           `json:"variant"`
	Task      string           `json:"task"`
	Test      string           `json:"test"`
	Arguments map[string]int32 `json:"args"`
}

type PerformanceAnalysisProxyServiceOptions struct {
	User    string
	Token   string
	BaseURL string
}
