package model

import (
	"crypto/sha1"
	"fmt"
	"hash"
	"io"
	"sort"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/anser/model"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const buildloggerCollection = "buildlogs"

func CreateLog(info LogInfo, artifact LogArtifactInfo) *Log {
	return &Log{
		ID:       info.ID(),
		Info:     info,
		Artifact: artifact,

		populated: true,
	}
}

// Log describes metadata for a buildlogger log.
type Log struct {
	ID          string          `bson:"_id,omitempty"`
	Info        LogInfo         `bson:"info,omitempty"`
	CreatedAt   time.Time       `bson:"created_at"`
	CompletedAt time.Time       `bson:"completed_at"`
	Artifact    LogArtifactInfo `bson:"artifact"`

	env       cedar.Environment
	populated bool
}

var (
	logIDKey          = bsonutil.MustHaveTag(Log{}, "ID")
	logInfoKey        = bsonutil.MustHaveTag(Log{}, "Info")
	logCreatedAtKey   = bsonutil.MustHaveTag(Log{}, "CreatedAt")
	logCompletedAtKey = bsonutil.MustHaveTag(Log{}, "CompletedAt")
	logArtifactKey    = bsonutil.MustHaveTag(Log{}, "Artifact")
)

// Setup sets the environment for the log. The environment is required for
// numerous functions on Log.
func (l *Log) Setup(e cedar.Environment) { l.env = e }

// IsNil returns if the log is populated or not.
func (l *Log) IsNil() bool { return !l.populated }

// Find searches the database for the log. The enviromemt should not be nil.
func (l *Log) Find() error {
	if l.env == nil {
		return errors.New("cannot find with a nil environment")
	}
	ctx, cancel := l.env.Context()
	defer cancel()

	if l.ID == "" {
		l.ID = l.Info.ID()
	}

	l.populated = false
	err := l.env.GetDB().Collection(buildloggerCollection).FindOne(ctx, bson.M{"_id": l.ID}).Decode(l)
	if db.ResultsNotFound(err) {
		return errors.New("could not find log record in the database")
	} else if err != nil {
		return errors.Wrap(err, "problem finding log record")
	}
	l.populated = true

	return nil
}

// Save upserts the log to the database. The log should be populated and the
// environment should not be nil.
func (l *Log) Save() error {
	if !l.populated {
		return errors.New("cannot save non-populated log data")
	}
	if l.env == nil {
		return errors.New("cannot save with a nil environment")
	}
	ctx, cancel := l.env.Context()
	defer cancel()

	if l.ID == "" {
		l.ID = l.Info.ID()
	}

	updateResult, err := l.env.GetDB().Collection(buildloggerCollection).ReplaceOne(ctx, bson.M{"_id": l.ID}, l, options.Replace().SetUpsert(true))
	grip.DebugWhen(err == nil, message.Fields{
		"ns":           model.Namespace{DB: l.env.GetConf().DatabaseName, Collection: buildloggerCollection},
		"id":           l.ID,
		"updateResult": updateResult,
		"artifact":     l.Artifact,
		"op":           "save buildlogger log",
	})
	return errors.Wrap(err, "problem saving log to collection")
}

// Remove removes the log from the database. The environment should not be nil.
func (l *Log) Remove() error {
	if l.env == nil {
		return errors.New("cannot remove with a nil environment")
	}
	ctx, cancel := l.env.Context()
	defer cancel()

	if l.ID == "" {
		l.ID = l.Info.ID()
	}

	_, err := l.env.GetDB().Collection(buildloggerCollection).DeleteOne(ctx, bson.M{"_id": l.ID})
	return errors.Wrapf(err, "problem removing log record with _id %s", l.ID)
}

// LogInfo describes information unique to a single buildlogger log.
type LogInfo struct {
	Project     string           `bson:"project,omitempty"`
	Version     string           `bson:"version,omitempty"`
	Variant     string           `bson:"variant,omitempty"`
	TaskName    string           `bson:"task_name,omitempty"`
	TaskID      string           `bson:"task_id,omitempty"`
	Execution   int              `bson:"execution"`
	TestName    string           `bson:"test_name,omitempty"`
	Trial       int              `bson:"trial"`
	ProcessName string           `bson:"proc_name,omitempty"`
	Format      string           `bson:"format,omitempty"`
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

// ID creates a unique hash for a buildlogger log.
func (id *LogInfo) ID() string {
	var hash hash.Hash

	if id.Schema == 0 {
		hash = sha1.New()
		_, _ = io.WriteString(hash, id.Project)
		_, _ = io.WriteString(hash, id.Version)
		_, _ = io.WriteString(hash, id.Variant)
		_, _ = io.WriteString(hash, id.TaskName)
		_, _ = io.WriteString(hash, id.TaskID)
		_, _ = io.WriteString(hash, fmt.Sprint(id.Execution))
		_, _ = io.WriteString(hash, id.TestName)
		_, _ = io.WriteString(hash, fmt.Sprint(id.Trial))
		_, _ = io.WriteString(hash, id.ProcessName)
		_, _ = io.WriteString(hash, id.Format)
		_, _ = io.WriteString(hash, fmt.Sprint(id.ExitCode))

		if len(id.Arguments) > 0 {
			args := []string{}
			for k, v := range id.Arguments {
				args = append(args, fmt.Sprintf("%s=%d", k, v))
			}

			sort.Strings(args)
			for _, str := range args {
				_, _ = io.WriteString(hash, str)
			}
		}
	} else {
		panic("unsupported schema")
	}

	return fmt.Sprintf("%x", hash.Sum(nil))
}
