package units

import (
	"context"
	"fmt"
	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/mongodb/grip"
	"gopkg.in/mgo.v2/bson"
	"time"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
)

type recalculateChangePointsJob struct {
	TaskID            string `bson:"task" json:"task" yaml:"task"`
	*job.Base         `bson:"metadata" json:"metadata" yaml:"metadata"`
	env               cedar.Environment
	maxConcurrentJobs int
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

type metricGrouping struct {
	Id struct {
		Project string
		Variant string
		Task    string
		Test    string
	} `bson:"_id"`
}

type measurementSeries struct {
	Measurement string `bson:"_id"`
	TimeSeries  struct {
		Id    string
		Value float64
		Order int
	}
}

func makeTimeSeriesPipeline(result metricGrouping) []bson.M {
	var pipeline []bson.M

	pipeline = append(pipeline, bson.M{
		"$match": bson.M{
			"info.project":   result.Id.Project,
			"info.variant":   result.Id.Variant,
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
			"value":       "$rollups.stats.val",
			"order":       "$info.order",
		},
	})

	pipeline = append(pipeline, bson.M{
		"$group": bson.M{
			"_id": "$measurement",
			"time_series": bson.M{
				"$push": bson.M{
					"id":    "$_id",
					"value": "$value",
					"order": "$order",
				},
			},
		},
	})

	return pipeline
}

func handle(ctx context.Context, result metricGrouping) {
}

func (j *recalculateChangePointsJob) Run(ctx context.Context) {
	defer j.MarkComplete()
	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}
	if j.maxConcurrentJobs == 0 {
		config := model.NewCedarConfig(j.env)
		err := config.Find()
		if err != nil {
			grip.Error("Unable to get cedar configuration")
			panic(err)
		}
	}
	collection := j.env.GetDB().Collection("recalculation_queue")
	cur, err := collection.Find(ctx, bson.M{})
	defer cur.Close(ctx)
	if err != nil {
		grip.Error("Unable to get metrics for recalculation of change points")
		panic(err)
	}
	for cur.Next(ctx) {
		var result metricGrouping
		err = cur.Decode(&result)
		if err != nil {
			grip.Error("Unable to decode recalculation metric.")
		}
		_, err := collection.DeleteOne(ctx, result)
		if err != nil {
			grip.Errorf("Unable to remove %q/%q/%q/%q from recalculation queue", result.Id.Project, result.Id.Variant, result.Id.Task, result.Id.Test)
		}
		go handle(ctx, result)
	}
}
