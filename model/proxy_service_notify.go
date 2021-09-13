package model

// PerformanceTestResultID is the ID to uniquely identify a new performance result available at test level.
type PerformanceTestResultID struct {
	Version   string           `bson:"version"`
	TaskID    string           `bson:"task_id"`
	Project   string           `bson:"project"`
	Variant   string           `bson:"variant"`
	Task      string           `bson:"task"`
	Test      string           `bson:"test"`
	Arguments map[string]int32 `bson:"args"`
}

type PerformanceAnalysisProxyServiceOptions struct {
	User    string
	Token   string
	BaseURL string
}
