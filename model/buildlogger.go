package model

import (
	"bytes"
	"context"
	"crypto/sha1"

	"fmt"
	"hash"
	"io"
	"sort"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/pail"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
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
			Version: 1,
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

// Find searches the DB for the log. The enviromemt should not be nil.
func (l *Log) Find(ctx context.Context) error {
	if l.env == nil {
		return errors.New("cannot find with a nil environment")
	}

	if l.ID == "" {
		l.ID = l.Info.ID()
	}

	l.populated = false
	if err := l.env.GetDB().Collection(buildloggerCollection).FindOne(ctx, bson.M{"_id": l.ID}).Decode(l); err != nil {
		return errors.Wrapf(err, "finding log record '%s'", l.ID)
	}
	l.populated = true

	return nil
}

// SaveNew saves a new log to the DB, if a log with the same ID already
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

	return errors.Wrapf(err, "saving new log '%s'", l.ID)
}

// Remove removes the log from the DB. The environment should not be nil.
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

	return errors.Wrapf(err, "removing log record '%s'", l.ID)
}

// Append uploads a chunk of log lines to the offline blob storage bucket
// configured for the log. The environment should not be nil.
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

	lineBuffer := &bytes.Buffer{}
	for _, line := range lines {
		// unlikely scenario, but just in case priority is out of range.
		if line.Priority > level.Emergency {
			line.Priority = level.Emergency
		} else if line.Priority < level.Invalid {
			line.Priority = level.Trace
		}

		_, err := lineBuffer.WriteString(prependPriorityAndTimestamp(line.Priority, line.Timestamp, line.Data))
		if err != nil {
			return errors.Wrap(err, "buffering lines")
		}
	}

	conf := &CedarConfig{}
	conf.Setup(l.env)
	if err := conf.Find(); err != nil {
		return errors.Wrap(err, "getting application configuration")
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
		return errors.Wrap(err, "creating bucket")
	}

	key := createBuildloggerChunkKey(lines[0].Timestamp, lines[len(lines)-1].Timestamp, len(lines))
	if err := bucket.Put(ctx, key, lineBuffer); err != nil {
		return errors.Wrap(err, "uploading log lines to bucket")
	}

	l.addToStatsCache(lines)

	return nil
}

func (l *Log) addToStatsCache(lines []LogLine) {
	linesSize := 0
	for _, line := range lines {
		linesSize += len(line.Data)
	}
	if linesSize == 0 {
		return
	}

	if err := l.env.GetStatsCache(cedar.StatsCacheBuildlogger).AddStat(cedar.Stat{
		Count:   linesSize,
		Project: l.Info.Project,
		Version: l.Info.Version,
		TaskID:  l.Info.TaskID,
	}); err != nil {
		grip.Error(message.WrapError(err, message.Fields{
			"message":  "stats were dropped",
			"log_info": l.Info,
			"cache":    cedar.StatsCacheBuildlogger,
		}))
	}
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
		return errors.Errorf("could not find log record '%s'", l.ID)
	}

	return errors.Wrapf(err, "closing log '%s'", l.ID)

}

// Download returns a LogIterator which iterates lines of the given log. The
// environment should not be nil.
func (l *Log) Download(ctx context.Context, timeRange TimeRange) (LogIterator, error) {
	if l.env == nil {
		return nil, errors.New("cannot download log with a nil environment")
	}

	if l.ID == "" {
		l.ID = l.Info.ID()
	}

	conf := &CedarConfig{}
	conf.Setup(l.env)
	if err := conf.Find(); err != nil {
		return nil, errors.Wrap(err, "getting application configuration")
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
		return nil, errors.Wrap(err, "creating bucket")
	}

	chunks, err := l.getChunks(ctx, bucket)
	if err != nil {
		return nil, errors.Wrap(err, "getting chunks")
	}

	return NewBatchedLogIterator(bucket, chunks, 2, timeRange), nil
}

func (l *Log) getChunks(ctx context.Context, bucket pail.Bucket) ([]LogChunkInfo, error) {
	var chunks []LogChunkInfo
	switch l.Artifact.Version {
	case 0:
		// Version 0 stores log chunk information directly in the
		// DB.
		chunks = l.Artifact.Chunks
	case 1:
		// Version 1 uses the key of the chunk in the pail-backed
		// offline storage to encode the chunk information.
		it, err := bucket.List(ctx, "")
		if err != nil {
			return nil, errors.Wrap(err, "listing chunks")
		}

		for it.Next(ctx) {
			chunk, err := parseBuildloggerChunkKey(it.Item().Name())
			if err != nil {
				return nil, errors.Wrapf(err, "parsing chunk key '%s'", it.Item().Name())
			}
			chunks = append(chunks, chunk)
		}
		if err = it.Err(); err != nil {
			return nil, errors.Wrap(err, "iterating chunks")
		}

		sort.Slice(chunks, func(i, j int) bool {
			return chunks[i].Start.Before(chunks[j].Start)
		})
	default:
		return nil, errors.Errorf("invalid artifact version %d", l.Artifact.Version)
	}

	return chunks, nil
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
	Priority  level.Priority
	Timestamp time.Time
	Data      string
}

// Logs describes a set of buildlogger logs, typically related by some
// criteria.
type Logs struct {
	Logs      []Log `bson:"results"`
	env       cedar.Environment
	populated bool
	timeRange TimeRange
}

// LogFindOptions describes the search criteria for the Find function on Logs.
type LogFindOptions struct {
	TimeRange       TimeRange
	Info            LogInfo
	Group           string
	EmptyTestName   bool
	LatestExecution bool
	Limit           int64
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
	findOpts.SetSort(bson.D{
		{Key: bsonutil.GetDottedKeyName(logInfoKey, logInfoExecutionKey), Value: -1},
		{Key: logCreatedAtKey, Value: -1},
	})
	it, err := l.env.GetDB().Collection(buildloggerCollection).Find(ctx, createFindQuery(opts), findOpts)
	if err != nil {
		return errors.WithStack(err)
	}

	if err = it.All(ctx, &l.Logs); err != nil {
		catcher := grip.NewBasicCatcher()
		catcher.Add(err)
		catcher.Add(it.Close(ctx))
		return catcher.Resolve()
	} else if err = it.Close(ctx); err != nil {
		return errors.WithStack(err)
	}
	if len(l.Logs) > 0 {
		l.timeRange = opts.TimeRange
		l.populated = true
	} else {
		return mongo.ErrNoDocuments
	}
	if opts.LatestExecution {
		l.filterLatestExecution()
	}

	return nil
}

func (l *Logs) filterLatestExecution() {
	execution := 0
	for i, log := range l.Logs {
		if log.Info.Execution < execution {
			l.Logs = l.Logs[:i]
			break
		} else {
			execution = log.Info.Execution
		}
	}
}

func createFindQuery(opts LogFindOptions) map[string]interface{} {
	search := bson.M{
		logCreatedAtKey:   bson.M{"$lte": opts.TimeRange.EndAt},
		logCompletedAtKey: bson.M{"$gte": opts.TimeRange.StartAt},
	}
	if opts.Info.Project != "" {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoProjectKey)] = opts.Info.Project
	}
	if opts.Info.Version != "" {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoVersionKey)] = opts.Info.Version
	}
	if opts.Info.Variant != "" {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoVariantKey)] = opts.Info.Variant
	}
	if opts.Info.TaskName != "" {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTaskNameKey)] = opts.Info.TaskName
	}
	if opts.Info.TaskID != "" {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTaskIDKey)] = opts.Info.TaskID
		delete(search, logCreatedAtKey)
		delete(search, logCompletedAtKey)
	} else {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoMainlineKey)] = true
	}
	if !opts.LatestExecution {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoExecutionKey)] = opts.Info.Execution
	}
	if opts.EmptyTestName {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTestNameKey)] = nil
	} else if opts.Info.TestName != "" {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTestNameKey)] = opts.Info.TestName
	}
	if opts.Info.Trial != 0 {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTrialKey)] = opts.Info.Trial
	}
	if opts.Info.ProcessName != "" {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoProcessNameKey)] = opts.Info.ProcessName
	}
	if opts.Info.Format != "" {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoFormatKey)] = opts.Info.Format
	}
	if opts.Group != "" && len(opts.Info.Tags) > 0 {
		search["$and"] = []bson.M{
			{bsonutil.GetDottedKeyName(logInfoKey, logInfoTagsKey): opts.Group},
			{bsonutil.GetDottedKeyName(logInfoKey, logInfoTagsKey): bson.M{"$in": opts.Info.Tags}},
		}
	} else if opts.Group != "" {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTagsKey)] = opts.Group
	} else if len(opts.Info.Tags) > 0 {
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoTagsKey)] = bson.M{"$in": opts.Info.Tags}
	}
	if len(opts.Info.Arguments) > 0 {
		var args []bson.M
		for key, val := range opts.Info.Arguments {
			args = append(args, bson.M{key: val})
		}
		search[bsonutil.GetDottedKeyName(logInfoKey, logInfoArgumentsKey)] = bson.M{"$in": args}
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
			return nil, errors.Wrap(catcher.Resolve(), "downloading log")
		}
		iterators = append(iterators, it)
	}

	return NewMergingIterator(iterators...), nil
}
