package model

import (
	"time"

	"github.com/mongodb/anser/bsonutil"
)

type Log struct {
	ID          string    `bson:"_id,omitempty"`
	Info        LogInfo   `bson:"info,omitempty"`
	CreatedAt   time.Time `bson:"created_at"`
	CompletedAt time.Time `bson:"completed_at"`

	Artifact LogArtifactInfo `bson:"artifact"`
}

var (
	logIDKey          = bsonutil.MustHaveTag(Log{}, "ID")
	logInfoKey        = bsonutil.MustHaveTag(Log{}, "Info")
	logCreatedAtKey   = bsonutil.MustHaveTag(Log{}, "CreatedAt")
	logCompletedAtKey = bsonutil.MustHaveTag(Log{}, "CompletedAt")
	logArtifactKey    = bsonutil.MustHaveTag(Log{}, "Artifact")
)

type LogInfo struct {
	Project     string           `bson:"project,omitempty"`
	Version     string           `bson:"version,omitempty"`
	Variant     string           `bson:"variant,omitempty"`
	TaskName    string           `bson:"task_name,omitempty"`
	TaskID      string           `bson:"task_id,omitempty"`
	Execution   int              `bson:"execution"`
	TestName    string           `bson:"test_name,omitempty"`
	Trial       int              `bson:"trial"`
	ProcessName string           `bson:proc_name,omitempty"`
	Arguments   map[string]int32 `bson:"args,omitempty"`
	ExitCode    int              `bson:"exit_code, omitempty"`
	Mainline    bool             `bson:"mainline"`
	Schema      int              `bson:"schema,omitempty"`
}

var (
	logInfoProjectKey     = bsonutil.MustHaveTag(LogInfo{}, "Project")
	logInfoVersionKey     = bsonutil.MustHaveTag(LogInfo{}, "Version")
	logInfoVariantKey     = bsonutil.MustHaveTag(LogInfo{}, "Variant")
	logInfoTaskNameKey    = bsonutil.MustHaveTag(LogInfo{}, "TaskName")
	logInfoTaskIDKey      = bsonutil.MustHaveTag(LogInfo{}, "TaskID")
	logInfoExecutionKey   = bsonutil.MustHaveTag(LogInfo{}, "Execution")
	logInfoTestNameKey    = bsonutil.MustHaveTag(LogInfo{}, "TestName")
	logInfoTrialKey       = bsonutil.MustHaveTag(LogInfo{}, "Trial")
	logInfoProcessNameKey = bsonutil.MustHaveTag(LogInfo{}, "ProcessName")
	logInfoFormatKey      = bsonutil.MustHaveTag(LogInfo{}, "Format")
	logInfoArgumentsKey   = bsonutil.MustHaveTag(LogInfo{}, "Arguments")
	logInfoExitCodeKey    = bsonutil.MustHaveTag(LogInfo{}, "ExitCode")
	logInfoMainlineKey    = bsonutil.MustHaveTag(LogInfo{}, "Mainline")
	logInfoSchemaKey      = bsonutil.MustHaveTag(LogInfo{}, "Schema")
)
