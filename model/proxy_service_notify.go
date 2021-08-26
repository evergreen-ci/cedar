package model

// PerformanceTestResultId is the Id to uniquely identify a new performance result available at test level
type PerformanceTestResultId struct {
	TaskID    string           `bson:"task_id"`
	Project   string           `bson:"project"`
	Variant   string           `bson:"variant"`
	Task      string           `bson:"task"`
	Test      string           `bson:"test"`
	Arguments map[string]int32 `bson:"args"`
}
