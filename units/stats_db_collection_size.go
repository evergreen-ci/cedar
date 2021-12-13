package units

import (
	"context"
	"fmt"

	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"gopkg.in/mgo.v2/bson"
)

const statsDBCollectionSizeJobName = "stats-db-collection-size"

type statsDBCollectionSizeJob struct {
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
	env      cedar.Environment
}

func init() {
	registry.AddJobType(statsDBCollectionSizeJobName,
		func() amboy.Job { return makeStatsDBCollectionSizeJob() })
}

func makeStatsDBCollectionSizeJob() *statsDBCollectionSizeJob {
	j := &statsDBCollectionSizeJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    statsDBCollectionSizeJobName,
				Version: 0,
			},
		},
		env: cedar.GetEnvironment(),
	}
	return j
}

// NewStatsDBCollectionSizeJob creates a new amboy job to collect the sizes of
// all collections in the cedar db and send them to splunk
func NewStatsDBCollectionSizeJob(env cedar.Environment, id string) amboy.Job {
	j := makeStatsDBCollectionSizeJob()
	j.SetID(fmt.Sprintf("%s.%s", statsDBCollectionSizeJobName, id))
	j.env = env
	return j
}

func (j *statsDBCollectionSizeJob) Run(ctx context.Context) {
	defer j.MarkComplete()
	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}
	db := j.env.GetDB()
	// We should be able to use bson.D{} but have to use primitive.D{}
	// to avoid WriteArray errors. Using bson.M gave the same error, in contrast
	// to GODRIVER-854.
	collectionNames, err := db.ListCollectionNames(ctx, primitive.D{})
	if err != nil {
		j.AddError(errors.Wrap(err, "getting collection names"))
		return
	}
	for _, collName := range collectionNames {
		var statsResult bson.M
		statsCmd := primitive.D{{Key: "collStats", Value: collName}}
		err := db.RunCommand(ctx, statsCmd).Decode(&statsResult)
		if err != nil {
			j.AddError(errors.Wrap(err, "getting collection stats"))
			return
		}
		grip.Info(message.Fields{
			"job_id":       j.ID(),
			"message":      statsDBCollectionSizeJobName,
			"collection":   collName,
			"storage_size": statsResult["storageSize"],
			"index_size":   statsResult["totalIndexSize"],
		})
	}
}
