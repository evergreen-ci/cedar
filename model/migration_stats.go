package model

import (
	"time"

	"github.com/mongodb/anser/bsonutil"
)

// MigrationStats represents statistics for migration jobs done with documents
// in the database. It should be used as a temporary sub-document within the
// documents in question.
type MigrationStats struct {
	MigratorID  string    `bson:"migrator_id"`
	StartedAt   time.Time `bson:"started_at"`
	CompletedAt time.Time `bson:"completed_at"`
	Version     int       `bson:"version"`
}

var (
	MigrationStatsMigratorIDKey  = bsonutil.MustHaveTag(MigrationStats{}, "MigratorID")
	MigrationStatsStartedAtKey   = bsonutil.MustHaveTag(MigrationStats{}, "StartedAt")
	MigrationStatsCompletedAtKey = bsonutil.MustHaveTag(MigrationStats{}, "CompletedAt")
	MigrationStatsVersionKey     = bsonutil.MustHaveTag(MigrationStats{}, "CompletedAt")
)
