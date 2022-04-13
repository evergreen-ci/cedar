package model

import (
	"context"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/anser/db"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
)

const (
	// BatchJobControllerCollection is the name of the DB collection
	// for batch job controller documents.
	BatchJobControllerCollection = "batch_job_controllers"
)

// BatchJobController represents a set of handles for controlling automatic
// batch jobs run in Cedar without requiring a deploy/restart of the
// application.
type BatchJobController struct {
	ID         string        `bson:"_id"`
	Collection string        `bson:"collection,omitempty"`
	BatchSize  int           `bson:"batch_size"`
	Iterations int           `bson:"iterations,omitempty"`
	Timeout    time.Duration `bson:"timeout,omitempty"`
	Version    int           `bson:"version"`
}

var (
	batchJobControllerIDKey         = bsonutil.MustHaveTag(BatchJobController{}, "ID")
	batchJobControllerCollectioKey  = bsonutil.MustHaveTag(BatchJobController{}, "Collection")
	batchJobControllerBatchSizeKey  = bsonutil.MustHaveTag(BatchJobController{}, "BatchSize")
	batchJobControllerIterationsKey = bsonutil.MustHaveTag(BatchJobController{}, "Iterations")
	batchJobControllerTimeoutKey    = bsonutil.MustHaveTag(BatchJobController{}, "Timeout")
	batchJobControllerVersionKey    = bsonutil.MustHaveTag(BatchJobController{}, "Version")
)

// FindBatchJobController searches the DB for the BatchJobController with
// the given ID.
func FindBatchJobController(ctx context.Context, env cedar.Environment, id string) (*BatchJobController, error) {
	var controller BatchJobController
	err := env.GetDB().Collection(BatchJobControllerCollection).FindOne(ctx, bson.M{batchJobControllerIDKey: id}).Decode(&controller)
	if db.ResultsNotFound(err) {
		return nil, errors.Wrapf(err, "batch job controller '%s' not found", id)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "finding batch job controller '%s'", id)
	}

	return &controller, nil
}
