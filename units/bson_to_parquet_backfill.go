package units

import (
	"context"
	"fmt"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TODO (EVG-16138): Remove this job once we do the BSON to Parquet cutover.

const bsonToParquetBackfillJobName = "bson-to-parquet-backfill"

type bsonToParquetBackfillJob struct {
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`

	env         cedar.Environment
	queue       amboy.Queue
	controller  *model.BatchJobController
	migrationID string
}

func init() {
	registry.AddJobType(bsonToParquetBackfillJobName, func() amboy.Job { return makeBSONToParquetBackfillJob() })
}

func makeBSONToParquetBackfillJob() *bsonToParquetBackfillJob {
	return &bsonToParquetBackfillJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    bsonToParquetBackfillJobName,
				Version: 1,
			},
		},
	}
}

// NewBSONToParquetBackfillJob returns a new Amboy job that does batched
// migrations of existing BSON test results to Apache Parquet test results
// stored in the Presto bucket.
func NewBSONToParquetBackfillJob(id string) amboy.Job {
	j := makeBSONToParquetBackfillJob()
	j.SetID(fmt.Sprintf("%s.%s", bsonToParquetBackfillJobName, id))

	return j
}

func (j *bsonToParquetBackfillJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}
	if j.queue == nil {
		j.queue = j.env.GetRemoteQueue()
	}

	controller, err := model.FindBatchJobController(ctx, j.env, bsonToParquetBackfillJobName)
	if err != nil {
		j.AddError(errors.Wrap(err, "getting batch job controller"))
		return
	}
	if controller.Iterations <= 0 {
		controller.Iterations = 1
	}
	if controller.Timeout <= 0 {
		controller.Timeout = time.Minute
	}
	j.controller = controller

	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, j.controller.Timeout)
	defer cancel()

	for i := 0; i < j.controller.Iterations; i++ {
		count, err := j.runIteration(ctx)
		if err != nil {
			j.AddError(err)
			return
		}
		if count == 0 {
			return
		}
	}
}

func (j *bsonToParquetBackfillJob) runIteration(ctx context.Context) (int64, error) {
	var count int64
	start := time.Now().UTC()
	collection := j.env.GetDB().Collection(j.controller.Collection)

	// This is really ugly, but unfortunately Mongo does not allow setting
	// limits on UpdateMany operations so we have to do a BulkWrite with
	// the batch size number of UpdateOne operations.
	updates := make([]mongo.WriteModel, j.controller.BatchSize)
	for i := 0; i < j.controller.BatchSize; i++ {
		updates[i] = mongo.NewUpdateOneModel().
			SetFilter(bson.M{"$or": []bson.M{
				{model.TestResultsMigrationKey: bson.M{"$exists": false}},
				{bsonutil.GetDottedKeyName(model.TestResultsMigrationKey, model.MigrationStatsVersionKey): bson.M{"$lt": j.controller.Version}},
				{"$and": []bson.M{
					{bsonutil.GetDottedKeyName(model.TestResultsMigrationKey, model.MigrationStatsStartedAtKey): bson.M{"$lte": time.Now().Add(-j.controller.Timeout)}},
					{bsonutil.GetDottedKeyName(model.TestResultsMigrationKey, model.MigrationStatsCompletedAtKey): nil},
				}},
			}}).
			SetUpdate(bson.M{"$set": bson.M{
				model.TestResultsMigrationKey: &model.MigrationStats{
					MigratorID: j.ID(),
					StartedAt:  &start,
					Version:    j.controller.Version,
				},
			}})
	}
	bulkWriteResult, err := collection.BulkWrite(ctx, updates)
	if err != nil {
		return count, errors.Wrap(err, "marking test results for migration in temporary collection")
	}
	if bulkWriteResult.MatchedCount == 0 {
		grip.Info(message.Fields{
			"job_name": bsonToParquetBackfillJobName,
			"message":  "test results BSON to Parquet backfill complete",
		})
		return count, nil
	}
	count = bulkWriteResult.MatchedCount

	cur, err := collection.Find(
		ctx,
		bson.M{
			bsonutil.GetDottedKeyName(model.TestResultsMigrationKey, model.MigrationStatsMigratorIDKey):  j.ID(),
			bsonutil.GetDottedKeyName(model.TestResultsMigrationKey, model.MigrationStatsCompletedAtKey): nil,
		},
		options.Find().SetLimit(int64(j.controller.BatchSize)),
	)
	if err != nil {
		return count, errors.Wrap(err, "getting test results from temporary collection")
	}
	defer func() {
		j.AddError(errors.Wrap(cur.Close(ctx), "closing DB cursor"))
	}()

	var results []model.TestResults
	if err = cur.All(ctx, &results); err != nil {
		return count, errors.Wrap(err, "decoding test results from temporary collection")
	}

	toUpdate := make([]string, len(results))
	for i, result := range results {
		result.Setup(j.env)
		if err = result.DownloadConvertAndWriteParquet(ctx); err != nil {
			return count, errors.Wrap(err, "downloading, converting, and writing Parquet test results")
		}

		toUpdate[i] = result.ID
	}

	updateResult, err := collection.UpdateMany(
		ctx,
		bson.M{"_id": bson.M{"$in": toUpdate}},
		bson.M{"$set": bson.M{
			bsonutil.GetDottedKeyName(model.TestResultsMigrationKey, model.MigrationStatsCompletedAtKey): time.Now(),
		}},
	)
	if err != nil {
		return count, errors.Wrap(err, "marking migrated test results complete in temporary collection")
	}
	if updateResult.ModifiedCount != int64(len(toUpdate)) {
		return count, errors.Errorf("expected to mark complete %d migrated test results documents from temporary collection, but marked %d instead", len(toUpdate), updateResult.ModifiedCount)
	}

	return count, nil
}
