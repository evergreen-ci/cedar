package model

import (
	"bytes"
	"context"
	"crypto/sha1"
	"fmt"
	"hash"
	"io"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/pail"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
)

const systemMetricsCollection = "system_metrics"

// SystemMetrics describes metadata for the system metrics data for
// a given task execution.
type SystemMetrics struct {
	ID          string                    `bson:"_id,omitempty"`
	Info        SystemMetricsInfo         `bson:"info,omitempty"`
	CreatedAt   time.Time                 `bson:"created_at"`
	CompletedAt time.Time                 `bson:"completed_at"`
	Artifact    SystemMetricsArtifactInfo `bson:"artifact"`

	env       cedar.Environment
	populated bool
}

var (
	systemMetricsIDKey          = bsonutil.MustHaveTag(SystemMetrics{}, "ID")
	systemMetricsInfoKey        = bsonutil.MustHaveTag(SystemMetrics{}, "Info")
	systemMetricsCreatedAtKey   = bsonutil.MustHaveTag(SystemMetrics{}, "CreatedAt")
	systemMetricsCompletedAtKey = bsonutil.MustHaveTag(SystemMetrics{}, "CompletedAt")
	systemMetricsArtifactKey    = bsonutil.MustHaveTag(SystemMetrics{}, "Artifact")
)

// CreateSystemMetrics is the entry point for creating the metadata for
// system metric time series data for a task execution.
func CreateSystemMetrics(info SystemMetricsInfo, options SystemMetricsArtifactOptions) *SystemMetrics {
	return &SystemMetrics{
		ID:        info.ID(),
		Info:      info,
		CreatedAt: time.Now(),
		Artifact: SystemMetricsArtifactInfo{
			Prefix:  info.ID(),
			Chunks:  []string{},
			Options: options,
		},
		populated: true,
	}
}

// Setup sets the environment for the system metrics object.
// The environment is required for numerous functions on SystemMetrics.
func (sm *SystemMetrics) Setup(e cedar.Environment) { sm.env = e }

// IsNil returns if the system metrics object is populated or not.
func (sm *SystemMetrics) IsNil() bool { return !sm.populated }

// SystemMetricsInfo describes information unique to the system metrics for a task.
type SystemMetricsInfo struct {
	Project   string `bson:"project,omitempty"`
	Version   string `bson:"version,omitempty"`
	Variant   string `bson:"variant,omitempty"`
	TaskName  string `bson:"task_name,omitempty"`
	TaskID    string `bson:"task_id,omitempty"`
	Execution int    `bson:"execution"`
	Mainline  bool   `bson:"mainline"`
	Schema    int    `bson:"schema,omitempty"`
}

var (
	systemMetricsInfoProjectKey   = bsonutil.MustHaveTag(SystemMetricsInfo{}, "Project")
	systemMetricsInfoVersionKey   = bsonutil.MustHaveTag(SystemMetricsInfo{}, "Version")
	systemMetricsInfoVariantKey   = bsonutil.MustHaveTag(SystemMetricsInfo{}, "Variant")
	systemMetricsInfoTaskNameKey  = bsonutil.MustHaveTag(SystemMetricsInfo{}, "TaskName")
	systemMetricsInfoTaskIDKey    = bsonutil.MustHaveTag(SystemMetricsInfo{}, "TaskID")
	systemMetricsInfoExecutionKey = bsonutil.MustHaveTag(SystemMetricsInfo{}, "Execution")
	systemMetricsInfoMainlineKey  = bsonutil.MustHaveTag(SystemMetricsInfo{}, "Mainline")
	systemMetricsInfoSchemaKey    = bsonutil.MustHaveTag(SystemMetricsInfo{}, "Schema")
)

// Find searches the database for the system metrics object. The environment should
// not be nil. Either the ID or full Info of the system metrics object needs to be
// specified.
func (sm *SystemMetrics) Find(ctx context.Context) error {
	if sm.env == nil {
		return errors.New("cannot find with a nil environment")
	}

	if sm.ID == "" {
		sm.ID = sm.Info.ID()
	}

	sm.populated = false
	err := sm.env.GetDB().Collection(systemMetricsCollection).FindOne(ctx, bson.M{"_id": sm.ID}).Decode(sm)
	if db.ResultsNotFound(err) {
		return fmt.Errorf("could not find system metrics record in the database with id %s", sm.ID)
	} else if err != nil {
		return errors.Wrapf(err, "problem finding system metrics with id %s", sm.ID)
	}

	sm.populated = true

	return nil
}

// Remove removes the system metrics record from the database. The environment should not be nil.
func (sm *SystemMetrics) Remove(ctx context.Context) error {
	if sm.env == nil {
		return errors.New("cannot remove a system metrics record with a nil environment")
	}

	if sm.ID == "" {
		sm.ID = sm.Info.ID()
	}

	deleteResult, err := sm.env.GetDB().Collection(systemMetricsCollection).DeleteOne(ctx, bson.M{"_id": sm.ID})
	grip.DebugWhen(err == nil, message.Fields{
		"collection":   systemMetricsCollection,
		"id":           sm.ID,
		"task_id":      sm.Info.TaskID,
		"execution":    sm.Info.Execution,
		"deleteResult": deleteResult,
		"op":           "remove system metrics record",
	})

	return errors.Wrapf(err, "problem removing system metrics record with _id %s", sm.ID)
}

// Append uploads a chunk of system metrics data to the offline blob storage bucket
// configured for the system metrics and updates the metadata in the database to reflect
// the uploaded data. The environment should not be nil.
func (sm *SystemMetrics) Append(ctx context.Context, data []byte) error {
	if sm.env == nil {
		return errors.New("cannot not append system metrics data with a nil environment")
	}
	if len(data) == 0 {
		grip.Warning(message.Fields{
			"collection": systemMetricsCollection,
			"id":         sm.ID,
			"task_id":    sm.Info.TaskID,
			"execution":  sm.Info.Execution,
			"message":    "append called with no system metrics data",
		})
		return nil
	}

	key := fmt.Sprint(utility.UnixMilli(time.Now()))

	conf := &CedarConfig{}
	conf.Setup(sm.env)
	if err := conf.Find(); err != nil {
		return errors.Wrap(err, "problem getting application configuration")
	}
	bucket, err := sm.Artifact.Options.Type.Create(
		ctx,
		sm.env,
		conf.Bucket.SystemMetricsBucket,
		sm.Artifact.Prefix,
		string(pail.S3PermissionsPrivate),
		true,
	)
	if err != nil {
		return errors.Wrap(err, "problem creating bucket")
	}
	if err := bucket.Put(ctx, key, bytes.NewReader(data)); err != nil {
		return errors.Wrap(err, "problem uploading system metrics data to bucket")
	}

	return errors.Wrap(sm.appendSystemMetricsChunkKey(ctx, key), "problem updating system metrics metadata during upload")
}

// appendSystemMetricsChunkKey adds a new key to the system metrics's chunks array in the
// database. The environment should not be nil.
func (sm *SystemMetrics) appendSystemMetricsChunkKey(ctx context.Context, key string) error {
	if sm.env == nil {
		return errors.New("cannot append to a system metrics object with a nil environment")
	}

	if sm.ID == "" {
		sm.ID = sm.Info.ID()
	}

	updateResult, err := sm.env.GetDB().Collection(systemMetricsCollection).UpdateOne(
		ctx,
		bson.M{"_id": sm.ID},
		bson.M{
			"$push": bson.M{
				bsonutil.GetDottedKeyName(systemMetricsArtifactKey, metricsArtifactInfoChunksKey): key,
			},
		},
	)
	grip.DebugWhen(err == nil, message.Fields{
		"collection":   systemMetricsCollection,
		"id":           sm.ID,
		"task_id":      sm.Info.TaskID,
		"execution":    sm.Info.Execution,
		"updateResult": updateResult,
		"key":          key,
		"op":           "append data chunk key to system metrics metadata",
	})
	if err == nil && updateResult.MatchedCount == 0 {
		err = errors.Errorf("could not find system metrics object with id %s in the database", sm.ID)
	}

	return errors.Wrapf(err, "problem appending system metrics data chunk to %s", sm.ID)
}

// ID creates a unique hash for the system metrics for a task.
func (id *SystemMetricsInfo) ID() string {
	var hash hash.Hash

	if id.Schema == 0 {
		hash = sha1.New()
		_, _ = io.WriteString(hash, id.Project)
		_, _ = io.WriteString(hash, id.Version)
		_, _ = io.WriteString(hash, id.Variant)
		_, _ = io.WriteString(hash, id.TaskName)
		_, _ = io.WriteString(hash, id.TaskID)
		_, _ = io.WriteString(hash, fmt.Sprint(id.Execution))
	} else {
		panic("unsupported schema")
	}

	return fmt.Sprintf("%x", hash.Sum(nil))
}
