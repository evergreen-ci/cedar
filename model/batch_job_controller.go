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
	batchJobControllerCollection = "batch_job_controller"
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
}

var (
	batchJobControllerIDKey         = bsonutil.MustHaveTag(BatchJobController{}, "ID")
	batchJobControllelCollectioKey  = bsonutil.MustHaveTag(BatchJobController{}, "Collection")
	batchJobControllelBatchSizeKey  = bsonutil.MustHaveTag(BatchJobController{}, "BatchSize")
	batchJobControllelIterationsKey = bsonutil.MustHaveTag(BatchJobController{}, "Iterations")
	batchJobControllelTimeoutKey    = bsonutil.MustHaveTag(BatchJobController{}, "Timeout")
)

// FindBatchJobController searches the database for the BatchJobController with
// the given ID.
func FindBatchJobController(ctx context.Context, env cedar.Environment, id string) (*BatchJobController, error) {
	var controller BatchJobController
	err := env.GetDB().Collection(batchJobControllerCollection).FindOne(ctx, bson.M{batchJobControllerIDKey: id}).Decode(&controller)
	if db.ResultsNotFound(err) {
		return nil, errors.Wrapf(err, "could not find batch job controller with id %s in the database", id)
	} else if err != nil {
		return nil, errors.Wrap(err, "finding batch job controller")
	}

	return &controller, nil
}
