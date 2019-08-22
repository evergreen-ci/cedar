package model

// APILog describes metadata for a buildlogger log.
type APILog struct {
	ID          APIString          `json:"id,omitempty"`
	Info        APILogInfo         `json:"info,omitempty"`
	CreatedAt   APITime            `json:"created_at"`
	CompletedAt APITime            `json:"completed_at"`
	Artifact    APILogArtifactInfo `json:"artifact"`
}

// APILogInfo describes information unique to a single buildlogger log.
type APILogInfo struct {
	Project     APIString         `bson:"project,omitempty"`
	Version     APIString         `bson:"version,omitempty"`
	Variant     APIString         `bson:"variant,omitempty"`
	TaskName    APIString         `bson:"task_name,omitempty"`
	TaskID      APIString         `bson:"task_id,omitempty"`
	Execution   int               `bson:"execution"`
	TestName    APIString         `bson:"test_name,omitempty"`
	Trial       int               `bson:"trial"`
	ProcessName APIString         `bson:"proc_name,omitempty"`
	Format      LogFormat         `bson:"format,omitempty"`
	Arguments   map[string]string `bson:"args,omitempty"`
	ExitCode    int               `bson:"exit_code, omitempty"`
	Mainline    bool              `bson:"mainline"`
	Schema      int               `bson:"schema,omitempty"`
}
