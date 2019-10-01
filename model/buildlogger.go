package model

import (
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
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const buildloggerCollection = "buildlogs"

// CreateLog is the entry point for creating a buildlogger Log.
func CreateLog(info LogInfo, artifactStorageType PailType) *Log {
	return &Log{
		ID:        info.ID(),
		Info:      info,
		CreatedAt: time.Now(),
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
		true,
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
func (l *Log) Close(ctx context.Context, exitCode int) error {
	if l.env == nil {
		return errors.New("cannot close log with a nil environment")
	}

	if l.ID == "" {
		l.ID = l.Info.ID()
	}

	completedAt := time.Now()
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
// environment should not be nil. When reverse is true, the log lines are
// returned in reverse order.
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
		false,
	)
	if err != nil {
		return nil, errors.Wrap(err, "problem creating bucket")
	}

	return NewBatchedLogIterator(bucket, l.Artifact.Chunks, 2, timeRange), nil
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
	Tags        []string          `bson:"tags,omitempty"`
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
	logInfoTagsKey        = bsonutil.MustHaveTag(LogInfo{}, "Tags")
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

		sort.Strings(id.Tags)
		for _, str := range id.Tags {
			_, _ = io.WriteString(hash, str)
		}

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

// Logs describes a set of buildlogger logs, typically related by some
// criteria.
type Logs struct {
	Logs      []Log `bson:"results"`
	env       cedar.Environment
	populated bool
	timeRange util.TimeRange
	reverse   bool
}

// EmptyLogInfo allows querying of null or missing fields.
type EmptyLogInfo struct {
	Project     bool
	Version     bool
	Variant     bool
	TaskName    bool
	TaskID      bool
	Execution   bool
	TestName    bool
	Trial       bool
	ProcessName bool
	Format      bool
	Tags        bool
	Arguments   bool
	ExitCode    bool
}

// LogFindOptions describes the search criteria for the Find function on Logs.
type LogFindOptions struct {
	TimeRange util.TimeRange
	Info      LogInfo
	Empty     EmptyLogInfo
	Limit     int64
	Reverse   bool
}

// Setup sets the environment for the logs. The environment is required for
// numerous methods on Logs.
func (l *Logs) Setup(e cedar.Environment) { l.env = e }

// IsNil returns if the logs are populated or not.
func (l *Logs) IsNil() bool { return l.populated }

// Find returns the logs matching the given search criteria. The environment
// should not be nil.
func (l *Logs) Find(ctx context.Context, opts LogFindOptions) error {
	if l.env == nil {
		return errors.New("cannot find with a nil env")
	}

	if opts.TimeRange.IsZero() || !opts.TimeRange.IsValid() {
		return errors.New("invalid time range given")
	}

	l.populated = false
	findOpts := options.Find()
	if opts.Limit > 0 {
		findOpts.SetLimit(opts.Limit)
	}
	findOpts.SetSort(bson.D{{logCreatedAtKey, -1}})
	it, err := l.env.GetDB().Collection(buildloggerCollection).Find(ctx, createFindQuery(opts), findOpts)
	if err != nil {
		return errors.WithStack(err)
	}
	if err = it.All(ctx, &l.Logs); err != nil {
		catcher := grip.NewBasicCatcher()
		catcher.Add(errors.WithStack(err))
		catcher.Add(errors.WithStack(it.Close(ctx)))
		return catcher.Resolve()
	} else if err = it.Close(ctx); err != nil {
		return errors.WithStack(err)
	}
	if len(l.Logs) > 0 {
		l.timeRange = opts.TimeRange
		l.reverse = opts.Reverse
		l.populated = true
	} else {
		return mongo.ErrNoDocuments
	}

	return nil
}

func createFindQuery(opts LogFindOptions) map[string]interface{} {
	search := bson.M{
		logCreatedAtKey:   bson.M{"$lte": opts.TimeRange.EndAt},
		logCompletedAtKey: bson.M{"$gte": opts.TimeRange.StartAt},
	}
	if opts.Info.Project != "" {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoProjectKey)] = opts.Info.Project
	} else if opts.Empty.Project {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoProjectKey)] = nil
	}
	if opts.Info.Version != "" {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoVersionKey)] = opts.Info.Version
	} else if opts.Empty.Version {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoVersionKey)] = nil
	}
	if opts.Info.Variant != "" {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoVariantKey)] = opts.Info.Variant
	} else if opts.Empty.Variant {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoVariantKey)] = nil
	}
	if opts.Info.TaskName != "" {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTaskNameKey)] = opts.Info.TaskName
	} else if opts.Empty.TaskName {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTaskNameKey)] = nil
	}
	if opts.Info.TaskID != "" {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTaskIDKey)] = opts.Info.TaskID
		delete(search, logCreatedAtKey)
		delete(search, logCompletedAtKey)
	} else if opts.Empty.TaskID {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTaskIDKey)] = nil
	} else {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoMainlineKey)] = true
	}
	if opts.Info.Execution != 0 {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoExecutionKey)] = opts.Info.Execution
	} else if opts.Empty.Execution {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoExecutionKey)] = 0
	}
	if opts.Info.TestName != "" {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTestNameKey)] = opts.Info.TestName
	} else if opts.Empty.TestName {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTestNameKey)] = nil
	}
	if opts.Info.Trial != 0 {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTrialKey)] = opts.Info.Trial
	} else if opts.Empty.Trial {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTrialKey)] = 0
	}
	if opts.Info.ProcessName != "" {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoProcessNameKey)] = opts.Info.ProcessName
	} else if opts.Empty.ProcessName {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoProcessNameKey)] = nil
	}
	if opts.Info.Format != "" {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoFormatKey)] = opts.Info.Format
	} else if opts.Empty.Format {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoFormatKey)] = nil
	}
	if len(opts.Info.Tags) > 0 {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTagsKey)] = bson.M{"$in": opts.Info.Tags}
	} else if opts.Empty.Tags {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTagsKey)] = nil
	}
	if opts.Info.ExitCode != 0 {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoExitCodeKey)] = opts.Info.ExitCode
	} else if opts.Empty.ExitCode {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoExitCodeKey)] = nil
	}
	if len(opts.Info.Arguments) > 0 {
		var args []bson.M
		for key, val := range opts.Info.Arguments {
			args = append(args, bson.M{key: val})
		}
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoArgumentsKey)] = bson.M{"$in": args}
	} else if opts.Empty.Arguments {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoArgumentsKey)] = nil
	}

	return search
}

// Merge merges the buildlogger logs, respecting the order of each line's
// timestamp. The logs should be populated and the environment should not be
// nil. When reverse is true, the log lines are returned in reverse order.
func (l *Logs) Merge(ctx context.Context) (LogIterator, error) {
	if !l.populated {
		return nil, errors.New("cannot merge unpopulated logs")
	}
	if l.env == nil {
		return nil, errors.New("cannot merge with a nil environment")
	}

	iterators := []LogIterator{}
	for i := range l.Logs {
		l.Logs[i].Setup(l.env)
		it, err := l.Logs[i].Download(ctx, l.timeRange)
		if err != nil {
			catcher := grip.NewBasicCatcher()
			for _, iterator := range iterators {
				catcher.Add(iterator.Close())
			}
			catcher.Add(err)
			return nil, errors.Wrap(catcher.Resolve(), "problem downloading log")
		}
		if l.reverse {
			err := it.Reverse()
			if err != nil {
				return nil, errors.Wrap(err, "problem reversing iterator")
			}
		}
		iterators = append(iterators, it)
	}

	it := NewMergingIterator(iterators...)
	if l.reverse {
		return it, errors.Wrap(it.Reverse(), "problem reversing iterator")
	}
	return it, nil
}
