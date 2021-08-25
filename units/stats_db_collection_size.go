package units

import (
	"context"
	"fmt"

	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
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
	j.SetDependency(dependency.NewAlways())
	return j
}

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

	collectionNames, err := db.ListCollectionNames(ctx, bson.D{{}})
	if err != nil {
		j.AddError(errors.Wrap(err, "Error getting collection names"))
	}
	var statsResult bson.M
	for _, collName := range collectionNames {

		statsCmd := bson.D{{Name: "collStats", Value: collName}}
		err := db.RunCommand(ctx, statsCmd).Decode(&statsResult)
		if err != nil {
			j.AddError(errors.Wrap(err, "Error getting collection stats"))
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