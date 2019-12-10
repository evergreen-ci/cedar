package units

import (
	"context"
	"fmt"
	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/grip"
	"gopkg.in/mgo.v2/bson"
	"time"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
)

type recalculateChangePointsJob struct {
	TaskID    string `bson:"task" json:"task" yaml:"task"`
	*job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
	env      cedar.Environment
}

func init() {
	registry.AddJobType("hello-world", helloWorldJobFactory)
}

func recalculateChangePointsJobFactory() amboy.Job {
	j := &recalculateChangePointsJob{
		Base: &job.Base{
			JobType: amboy.JobType{
				Name:    "hello-world",
				Version: 1,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func NewRecalculateChangePointsJob(name string) amboy.Job {
	j := recalculateChangePointsJobFactory().(*recalculateChangePointsJob)
	j.SetID(fmt.Sprintf("%s-%d-%s-%s", j.Type().Name, job.GetNumber(),
		time.Now().Format("2006-01-02::15.04.05"), name))
	j.TaskID = name
	return j
}

func (j *recalculateChangePointsJob) Run(ctx context.Context) {
	defer j.MarkComplete()
	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}
	cur, err := j.env.GetDB().Collection("recalculation_queue").Find(ctx, bson.M{})
	defer cur.Close(ctx)
	if err != nil {
		grip.Errorf("Unable to get metrics for recalculation of change points: %q", err)
		panic(err)
	}
	for cur.Next(ctx) {
		var result struct{
			Id struct {
				Project string
				Variant string
				Task string
				Test string
			} `bson:"_id"`
		}
		err = cur.Decode(&result)
		if err != nil {
			grip.Errorf("Unable to decode recalculation metric.")
			panic(err)
		}

		var pipeline []bson.M

		pipeline = append(pipeline, bson.M{
			"$match": bson.M{
				"info.project": result.Id.Project,
				"info.variant": result.Id.Variant,
				"info.task_name": result.Id.Task,
				"info.test_name": result.Id.Test,
			},
		})

		pipeline = append(pipeline, bson.M{
			"$match": bson.M{
				"info.order": bson.M{
					"$exists": true,
				},
				"info.mainline": true,
			},
		})

		pipeline = append(pipeline, bson.M{
			"$unwind": "$rollups.stats",
		})

		pipeline = append(pipeline, bson.M{
			"$project": bson.M{
				"measurement": "$rollups.stats.name",
				"value": "$rollups.stats.val",
				"order": "$info.order",
			},
		})

		pipeline = append(pipeline, bson.M{
			"$group": bson.M{
				"_id": "$measurement",
				"time_series": bson.M{
					"$push": bson.M{
						"id": "$_id",
						"value": "$value",
						"order": "$order",
					},
				},
			},
		})
		
		async for result in cedar.perf_results.aggregate(pipeline):
		yield {
		"measurement": result["_id"],
		"data": list(
		map(
		lambda x: x["value"],
		sorted(result["time_series"], key=lambda x: x["order"]),
		)
		),
		"ids": list(
		map(
		lambda x: x["id"],
		sorted(result["time_series"], key=lambda x: x["order"]),
		)
		),
		}
	}
}
