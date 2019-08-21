package model

import (
	"container/heap"
	"context"
	"crypto/sha1"
	"fmt"
	"hash"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/pail"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/recovery"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
)

const buildloggerCollection = "buildlogs"

// CreateLog is the entry point for creating a buildlogger Log.
func CreateLog(info LogInfo, artifactStorageType PailType) *Log {
	return &Log{
		ID:   info.ID(),
		Info: info,
		Artifact: LogArtifactInfo{
			Type:    artifactStorageType,
			Prefix:  info.ID(),
			Version: 0,
		},
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
func (l *Log) Find(ctx context.Context) error {
	if l.env == nil {
		return errors.New("cannot find with a nil environment")
	}

	if l.ID == "" {
		l.ID = l.Info.ID()
	}

	l.populated = false
	err := l.env.GetDB().Collection(buildloggerCollection).FindOne(ctx, bson.M{"_id": l.ID}).Decode(l)
	if db.ResultsNotFound(err) {
		return errors.Wrapf(err, "could not find log record with id %s in the database", l.ID)
	} else if err != nil {
		return errors.Wrap(err, "problem finding log record")
	}
	l.populated = true

	return nil
}

// SaveNew saves a new log to the database, if a log with the same ID already
// exists an error is returned. The log should be populated and the environment
// should not be nil.
func (l *Log) SaveNew(ctx context.Context) error {
	if !l.populated {
		return errors.New("cannot save unpopulated log")
	}
	if l.env == nil {
		return errors.New("cannot save with a nil environment")
	}

	if l.ID == "" {
		l.ID = l.Info.ID()
	}

	insertResult, err := l.env.GetDB().Collection(buildloggerCollection).InsertOne(ctx, l)
	grip.DebugWhen(err == nil, message.Fields{
		"collection":   buildloggerCollection,
		"id":           l.ID,
		"insertResult": insertResult,
		"op":           "save new buildlogger log",
	})

	return errors.Wrapf(err, "problem saving new log %s", l.ID)
}

// Remove removes the log from the database. The environment should not be nil.
func (l *Log) Remove(ctx context.Context) error {
	if l.env == nil {
		return errors.New("cannot remove a log with a nil environment")
	}

	if l.ID == "" {
		l.ID = l.Info.ID()
	}

	deleteResult, err := l.env.GetDB().Collection(buildloggerCollection).DeleteOne(ctx, bson.M{"_id": l.ID})
	grip.DebugWhen(err == nil, message.Fields{
		"collection":   buildloggerCollection,
		"id":           l.ID,
		"deleteResult": deleteResult,
		"op":           "remove log",
	})

	return errors.Wrapf(err, "problem removing log record with _id %s", l.ID)
}

// Append uploads a chunk of log lines to the offline blob storage bucket
// configured for the log and updates the metadata in the database to reflect
// the uploaded lines. The environment should not be nil.
func (l *Log) Append(ctx context.Context, lines []LogLine) error {
	if l.env == nil {
		return errors.New("cannot not append log lines with a nil environment")
	}
	if len(lines) == 0 {
		grip.Warning(message.Fields{
			"collection": buildloggerCollection,
			"id":         l.ID,
			"message":    "append called with no log lines",
		})
		return nil
	}
	key := fmt.Sprint(util.UnixMilli(time.Now()))

	linesCombined := ""
	for _, line := range lines {
		linesCombined += prependTimestamp(line.Timestamp, line.Data)
	}

	conf := &CedarConfig{}
	conf.Setup(l.env)
	if err := conf.Find(); err != nil {
		return errors.Wrap(err, "problem getting application configuration")
	}
	bucket, err := l.Artifact.Type.Create(
		ctx,
		l.env,
		conf.Bucket.BuildLogsBucket,
		l.Artifact.Prefix,
		string(pail.S3PermissionsPrivate),
	)
	if err != nil {
		return errors.Wrap(err, "problem creating bucket")
	}
	if err := bucket.Put(ctx, key, strings.NewReader(linesCombined)); err != nil {
		return errors.Wrap(err, "problem uploading log lines to bucket")
	}

	info := LogChunkInfo{
		Key:      key,
		NumLines: len(lines),
		Start:    lines[0].Timestamp,
		End:      lines[len(lines)-1].Timestamp,
	}
	return errors.Wrap(l.appendLogChunkInfo(ctx, info), "problem updating log metadata during upload")
}

// appendLogChunkInfo adds a new log chunk to the log's chunks array in the
// database. The environment should not be nil.
func (l *Log) appendLogChunkInfo(ctx context.Context, logChunk LogChunkInfo) error {
	if l.env == nil {
		return errors.New("cannot append to a log with a nil environment")
	}

	if l.ID == "" {
		l.ID = l.Info.ID()
	}

	updateResult, err := l.env.GetDB().Collection(buildloggerCollection).UpdateOne(
		ctx,
		bson.M{"_id": l.ID},
		bson.M{
			"$push": bson.M{
				bsonutil.GetDottedKeyName(logArtifactKey, logArtifactInfoChunksKey): logChunk,
			},
		},
	)
	grip.DebugWhen(err == nil, message.Fields{
		"collection":   buildloggerCollection,
		"id":           l.ID,
		"updateResult": updateResult,
		"logChunkInfo": logChunk,
		"op":           "append log chunk info to buildlogger log",
	})
	if err == nil && updateResult.MatchedCount == 0 {
		err = errors.Errorf("could not find log record with id %s in the database", l.ID)
	}

	return errors.Wrapf(err, "problem appending log chunk info to %s", l.ID)
}

// Close "closes out" the log by populating the completed_at and info.exit_code
// fields. The environment should not be nil.
func (l *Log) Close(ctx context.Context, completedAt time.Time, exitCode int) error {
	if l.env == nil {
		return errors.New("cannot close log with a nil environment")
	}

	if l.ID == "" {
		l.ID = l.Info.ID()
	}

	updateResult, err := l.env.GetDB().Collection(buildloggerCollection).UpdateOne(
		ctx,
		bson.M{"_id": l.ID},
		bson.M{
			"$set": bson.M{
				logCompletedAtKey: completedAt,
				bsonutil.GetDottedKeyName(logInfoKey, logInfoExitCodeKey): exitCode,
			},
		},
	)
	grip.DebugWhen(err == nil, message.Fields{
		"collection":   buildloggerCollection,
		"id":           l.ID,
		"completed_at": completedAt,
		"exit_code":    exitCode,
		"updateResult": updateResult,
		"op":           "close buildlogger log",
	})
	if err == nil && updateResult.MatchedCount == 0 {
		err = errors.Errorf("could not find log record with id %s in the database", l.ID)
	}

	return errors.Wrapf(err, "problem closing log with id %s", l.ID)

}

// Download returns a LogIterator which iterates lines of the given log. The
// environment should not be nil.
func (l *Log) Download(ctx context.Context, timeRange util.TimeRange) (LogIterator, error) {
	if l.env == nil {
		return nil, errors.New("cannot download log with a nil environment")
	}

	if l.ID == "" {
		l.ID = l.Info.ID()
	}

	conf := &CedarConfig{}
	conf.Setup(l.env)
	if err := conf.Find(); err != nil {
		return nil, errors.Wrap(err, "problem getting application configuration")
	}

	bucket, err := l.Artifact.Type.Create(
		ctx,
		l.env,
		conf.Bucket.BuildLogsBucket,
		l.Artifact.Prefix,
		string(pail.S3PermissionsPrivate),
	)
	if err != nil {
		return nil, errors.Wrap(err, "problem creating bucket")
	}

	return NewBatchedLogIterator(bucket, l.Artifact.Chunks, 100, timeRange), nil
}

// MergeLogs merges N buildlogger logs, passed in as LogIterators, respecting
// the order of each line's timestamp. Note that once all lines are merged, the
// returned string channel is closed.
func MergeLogs(ctx context.Context, iterators ...LogIterator) chan string {
	h := &LogIteratorHeap{}
	heap.Init(h)
	lines := make(chan string, len(iterators))

	for _, it := range iterators {
		if it.Next(ctx) {
			h.SafePush(it)
		}
	}

	go func() {
		defer recovery.LogStackTraceAndContinue("merging buildlogger logs")
		defer close(lines)
		for h.Len() > 0 {
			if ctx.Err() != nil {
				return
			}

			it := h.SafePop()

			line := fmt.Sprintf("%s %s", it.Item().Timestamp, it.Item().Data)
			lines <- line

			if it.Next(ctx) {
				h.SafePush(it)
			}
		}
	}()

	return lines
}

// LogInfo describes information unique to a single buildlogger log.
type LogInfo struct {
	Project     string            `bson:"project,omitempty"`
	Version     string            `bson:"version,omitempty"`
	Variant     string            `bson:"variant,omitempty"`
	TaskName    string            `bson:"task_name,omitempty"`
	TaskID      string            `bson:"task_id,omitempty"`
	Execution   int               `bson:"execution"`
	TestName    string            `bson:"test_name,omitempty"`
	Trial       int               `bson:"trial"`
	ProcessName string            `bson:"proc_name,omitempty"`
	Format      LogFormat         `bson:"format,omitempty"`
	Arguments   map[string]string `bson:"args,omitempty"`
	ExitCode    int               `bson:"exit_code, omitempty"`
	Mainline    bool              `bson:"mainline"`
	Schema      int               `bson:"schema,omitempty"`
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
		_, _ = io.WriteString(hash, string(id.Format))

		if len(id.Arguments) > 0 {
			args := []string{}
			for k, v := range id.Arguments {
				args = append(args, fmt.Sprintf("%s=%s", k, v))
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

// LogLine describes a buildlogger log line. This is an intermediary type that
// passes data from RPC calls to the upload phase and is used as the return
// item for the LogIterator.
type LogLine struct {
	Timestamp time.Time
	Data      string
}
