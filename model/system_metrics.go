package model

import (
	"context"
	"crypto/sha1"
	"fmt"
	"hash"
	"io"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
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
// system metric time series data for a task execution. User specified
// Prefix and Key will be written over.
func CreateSystemMetrics(info SystemMetricsInfo, options SystemMetricsArtifactOptions) *SystemMetrics {
	return &SystemMetrics{
		ID:        info.ID(),
		Info:      info,
		CreatedAt: time.Now(),
		Artifact: SystemMetricsArtifactInfo{
			Prefix:  info.ID(),
			Keys:    []string{},
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
func (result *SystemMetrics) Find(ctx context.Context) error {
	if result.env == nil {
		return errors.New("cannot find with a nil environment")
	}

	if result.ID == "" {
		result.ID = result.Info.ID()
	}

	result.populated = false
	err := result.env.GetDB().Collection(systemMetricsCollection).FindOne(ctx, bson.M{"_id": result.ID}).Decode(result)
	if db.ResultsNotFound(err) {
		return fmt.Errorf("could not find system metrics record in the database with id %s", result.ID)
	} else if err != nil {
		return errors.Wrapf(err, "problem finding system metrics with id %s", result.ID)
	}

	result.populated = true

	return nil
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
