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
)

const buildloggerCollection = "buildlogger"

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

func (l *Log) Setup(e cedar.Environment) { l.env = e }
func (l *Log) IsNil() bool               { return !l.populated }
func (l *Log) Find() error {
	conf, session, err := cedar.GetSessionWithConfig(l.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	l.populated = false
	err = session.DB(conf.DatabaseName).C(buildloggerCollection).FindId(l.ID).One(l)
	if db.ResultsNotFound(err) {
		return errors.New("could not find log record in the database")
	} else if err != nil {
		return errors.Wrap(err, "problem finding log record")
	}

	l.populated = true

	return nil
}

func (l *Log) Save() error {
	if !l.populated {
		return errors.New("cannot save non-populated log data")
	}

	if l.ID == "" {
		l.ID = l.Info.ID()
	}

	conf, session, err := cedar.GetSessionWithConfig(l.env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()

	changeInfo, err := session.DB(conf.DatabaseName).C(buildloggerCollection).UpsertId(l.ID, l)
	grip.DebugWhen(err == nil, message.Fields{
		"ns":         model.Namespace{DB: conf.DatabaseName, Collection: buildloggerCollection},
		"id":         l.ID,
		"updated":    changeInfo.Updated,
		"upsertedID": changeInfo.UpsertedId,
		"artifact":   l.Artifact,
		"op":         "save buildlogger log",
	})
	return errors.Wrap(err, "problem saving log to collection")
}

func (l *Log) Remove() error {
	if l.ID == "" {
		l.ID = l.Info.ID()
	}

	conf, session, err := cedar.GetSessionWithConfig(l.env)
	if err != nil {
		errors.WithStack(err)
	}
	defer session.Close()

	return errors.Wrapf(session.DB(conf.DatabaseName).C(perflCollection).RemoveId(l.ID),
		"problem removing log with _id %s", l.ID)
}

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
		_, _ = io.WriteString(hash, Format)
		_, _ = io.WriteString(hash, fmt.Sprintf(id.ExitCode))

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
