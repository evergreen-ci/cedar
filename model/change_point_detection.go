package model

import (
	"context"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
)

type PerformanceResultSeriesId struct {
	Project string `bson:"project"`
	Variant string `bson:"variant"`
	Task    string `bson:"task"`
	Test    string `bson:"test"`
}

type TimeSeriesEntry struct {
	PerfResultID string  `bson:"perf_result_id"`
	Value        float64 `bson:"value"`
	Order        int     `bson:"order"`
}

type MeasurementData struct {
	Measurement string            `bson:"measurement"`
	TimeSeries  []TimeSeriesEntry `bson:"time_series"`
}

type PerformanceData struct {
	PerformanceResultId PerformanceResultSeriesId `bson:"_id"`
	Data                []MeasurementData         `bson:"data"`
}

func MarkPerformanceResultsAsAnalyzed(ctx context.Context, env cedar.Environment, id PerformanceResultSeriesId) error {
	_, err := env.GetDB().Collection(perfResultCollection).UpdateMany(ctx, id, bson.M{
		"$currentDate": bson.M{
			"change_points_detected_at": true,
		},
	})
	if err != nil {
		return errors.Wrapf(err, "Unable to mark performance results as analyzed for change points")
	}
	return nil
}

func GetPerformanceResultSeriesIdsNeedingChangePointDetection(ctx context.Context, env cedar.Environment) ([]PerformanceResultSeriesId, error) {
	cur, err := env.GetDB().Collection(perfResultCollection).Aggregate(ctx, []bson.M{
		{
			"$match": bson.M{
				"info.order":     bson.M{"$exists": true},
				"info.mainline":  true,
				"info.project":   bson.M{"$exists": true},
				"info.variant":   bson.M{"$exists": true},
				"info.task_name": bson.M{"$exists": true},
				"info.test_name": bson.M{"$exists": true},
			},
		},
		{
			"$match": bson.M{
				"$expr": bson.M{
					"$lt": []string{
						"$rollups.change_points_detected_at",
						"$rollups.processed_at",
					},
				},
			},
		},
		{
			"$group": bson.M{
				"_id": bson.M{
					"task":    "$info.task_name",
					"variant": "$info.variant",
					"project": "$info.project",
					"test":    "$info.test_name",
				},
			},
		},
		{
			"$replaceRoot": bson.M{
				"newRoot": "_id",
			},
		},
	})
	if err != nil {
		return nil, errors.Wrapf(err, "Unable to get metrics needing change point detection")
	}
	defer cur.Close(ctx)
	var res []PerformanceResultSeriesId
	err = cur.All(ctx, &res)
	if err != nil {
		return nil, errors.Wrapf(err, "Unable to decode metrics needing change results")
	}
	return res, nil
}

func GetPerformanceResultSeriesIds(ctx context.Context, env cedar.Environment) ([]PerformanceResultSeriesId, error) {
	cur, err := env.GetDB().Collection(perfResultCollection).Aggregate(ctx, []bson.M{
		{
			"$match": bson.M{
				"info.order":     bson.M{"$exists": true},
				"info.mainline":  true,
				"info.project":   bson.M{"$exists": true},
				"info.variant":   bson.M{"$exists": true},
				"info.task_name": bson.M{"$exists": true},
				"info.test_name": bson.M{"$exists": true},
			},
		},
		{
			"$unwind": "$rollups.stats",
		},
		{
			"$group": bson.M{
				"_id": bson.M{
					"task":    "$info.task_name",
					"variant": "$info.variant",
					"project": "$info.project",
					"test":    "$info.test_name",
				},
			},
		},
		{
			"$replaceRoot": bson.M{
				"newRoot": "$_id",
			},
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "Cannot aggregate time series ids")
	}
	defer cur.Close(ctx)
	var res []PerformanceResultSeriesId
	err = cur.All(ctx, &res)
	if err != nil {
		return nil, errors.Wrap(err, "Could not decode time series ids")
	}
	return res, nil
}
func GetPerformanceData(ctx context.Context, env cedar.Environment, performanceResultId PerformanceResultSeriesId) (*PerformanceData, error) {
	cur, err := env.GetDB().Collection(perfResultCollection).Aggregate(ctx, []bson.M{
		{
			"$match": bson.M{
				"info.order": bson.M{
					"$exists": true,
				},
				"info.mainline":  true,
				"info.project":   performanceResultId.Project,
				"info.variant":   performanceResultId.Variant,
				"info.task_name": performanceResultId.Task,
				"info.test_name": performanceResultId.Test,
			},
		},
		{
			"$unwind": "$rollups.stats",
		},
		{
			"$group": bson.M{
				"_id": bson.M{
					"task":        "$info.task_name",
					"variant":     "$info.variant",
					"project":     "$info.project",
					"test":        "$info.test_name",
					"measurement": "$rollups.stats.name",
				},
				"time_series": bson.M{
					"$push": bson.M{
						"value":          "$rollups.stats.val",
						"order":          "$info.order",
						"perf_result_id": "$_id",
					},
				},
			},
		},
		{
			"$group": bson.M{
				"_id": bson.M{
					"project": "$_id.project",
					"variant": "$_id.variant",
					"task":    "$_id.task",
					"test":    "$_id.test",
				},
				"measurements": bson.M{
					"$push": bson.M{
						"measurement": "$_id.measurement",
						"time_series": "$time_series",
					},
				},
			},
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "Cannot aggregate time series")
	}
	defer cur.Close(ctx)
	var res PerformanceData
	err = cur.All(ctx, &res)
	if err != nil {
		return nil, errors.Wrap(err, "Could not decode time series")
	}
	return &res, nil
}

func ReplaceChangePoints(ctx context.Context, env cedar.Environment, performanceData *PerformanceData, mappedChangePoints map[string][]ChangePoint) error {
	err := clearChangePoints(ctx, env, performanceData.PerformanceResultId)
	if err != nil {
		return errors.Wrapf(err, "Unable to clear change points for measurement %s", performanceData.PerformanceResultId)
	}
	catcher := grip.NewBasicCatcher()
	for _, measurementData := range performanceData.Data {
		changePoints := mappedChangePoints[measurementData.Measurement]
		for _, cp := range changePoints {
			perfResultId := measurementData.TimeSeries[cp.Index].PerfResultID
			err = createChangePoint(ctx, env, perfResultId, measurementData.Measurement, cp.Algorithm)
			if err != nil {
				catcher.Add(errors.Wrapf(err, "Failed to update performance result with change point %s", perfResultId))
			}
		}
	}
	return catcher.Resolve()
}

func clearChangePoints(ctx context.Context, env cedar.Environment, performanceResultId PerformanceResultSeriesId) error {
	seriesFilter := bson.M{
		"info.project":   performanceResultId.Project,
		"info.variant":   performanceResultId.Variant,
		"info.task_name": performanceResultId.Task,
		"info.test_name": performanceResultId.Test,
	}
	clearingUpdate := bson.M{
		"set": bson.M{
			"change_points": bson.A{},
		},
	}
	_, err := env.GetDB().Collection(perfResultCollection).UpdateMany(ctx, seriesFilter, clearingUpdate)
	return errors.Wrap(err, "Unable to clear change points")
}

func createChangePoint(ctx context.Context, env cedar.Environment, resultToUpdate string, measurement string, algorithm AlgorithmInfo) error {
	filter := bson.M{"_id": resultToUpdate}
	update := bson.M{
		"$push": bson.M{
			"change_points": ChangePoint{
				Measurement:  measurement,
				Algorithm:    algorithm,
				CalculatedOn: time.Now(),
			},
		},
	}
	_, err := env.GetDB().Collection(perfResultCollection).UpdateOne(ctx, filter, update)
	return errors.Wrap(err, "Unable to create change point")
}
