package model

import (
	"context"
	"fmt"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/anser/bsonutil"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
)

// PerfAnalysis contains information about when the associated performance
// result was analyzed for change points.
type PerfAnalysis struct {
	ProcessedAt time.Time `bson:"processed_at" json:"processed_at" yaml:"processed_at"`
}

var (
	perfAnalysisProcessedAtKey = bsonutil.MustHaveTag(PerfAnalysis{}, "ProcessedAt")
)

// PerformanceResultSeriesID represents the set of fields used identify a
// series of performance results for change point detection.
type PerformanceResultSeriesID struct {
	Project     string               `bson:"project"`
	Variant     string               `bson:"variant"`
	Task        string               `bson:"task"`
	Test        string               `bson:"test"`
	Measurement string               `bson:"measurement"`
	Arguments   PerformanceArguments `bson:"args"`
}

// String creates a string representation of a performance result series ID.
func (p PerformanceResultSeriesID) String() string {
	return fmt.Sprintf("%s %s %s %s %v", p.Project, p.Variant, p.Task, p.Test, p.Arguments)
}

// TimeSeriesEntry represents information about a single performance result in
// a series.
type TimeSeriesEntry struct {
	PerfResultID string  `bson:"perf_result_id"`
	Value        float64 `bson:"value"`
	Order        int     `bson:"order"`
	Version      string  `bson:"version"`
}

// PerformanceData represents information about a performance result series and
// its associated measurement data.
type PerformanceData struct {
	PerformanceResultId PerformanceResultSeriesID `bson:"_id"`
	TimeSeries          []TimeSeriesEntry         `bson:"time_series"`
}

// MarkPerformanceResultsAsAnalyzed marks all the performance results with the
// given series ID as analyzed with the current date timestamp.
func MarkPerformanceResultsAsAnalyzed(ctx context.Context, env cedar.Environment, performanceResultId PerformanceResultSeriesID) error {
	filter := bson.M{
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoProjectKey):   performanceResultId.Project,
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVariantKey):   performanceResultId.Variant,
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskNameKey):  performanceResultId.Task,
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTestNameKey):  performanceResultId.Test,
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoArgumentsKey): performanceResultId.Arguments,
	}

	update := bson.M{
		"$currentDate": bson.M{
			bsonutil.GetDottedKeyName(perfAnalysisKey, perfAnalysisProcessedAtKey): true,
		},
	}
	_, err := env.GetDB().Collection(perfResultCollection).UpdateMany(ctx, filter, update)
	if err != nil {
		return errors.Wrapf(err, "marking performance results as analyzed for change points")
	}
	return nil
}

// GetPerformanceResultSeriesIdsNeedingTimeSeriesUpdate queries the database
// and gets all the performance result series IDs that contain results that
// have not yet been analyzed.
func GetPerformanceResultSeriesIdsNeedingTimeSeriesUpdate(ctx context.Context, env cedar.Environment) ([]PerformanceResultSeriesID, error) {
	cur, err := env.GetDB().Collection(perfResultCollection).Aggregate(ctx, []bson.M{
		{
			"$match": bson.M{
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoOrderKey):    bson.M{"$exists": true},
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoMainlineKey): true,
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoProjectKey):  bson.M{"$exists": true},
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVariantKey):  bson.M{"$exists": true},
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskNameKey): bson.M{"$exists": true},
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTestNameKey): bson.M{"$exists": true},
				bsonutil.GetDottedKeyName(perfRollupsKey, perfRollupsStatsKey):    bson.M{"$not": bson.M{"$size": 0}},
			},
		},
		{
			"$match": bson.M{
				"$expr": bson.M{
					"$lt": []string{
						"$" + bsonutil.GetDottedKeyName(perfAnalysisKey, perfAnalysisProcessedAtKey),
						"$" + bsonutil.GetDottedKeyName(perfRollupsKey, perfRollupsProcessedAtKey),
					},
				},
			},
		},
		{
			"$group": bson.M{
				"_id": bson.M{
					"project": "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoProjectKey),
					"variant": "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVariantKey),
					"task":    "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskNameKey),
					"test":    "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTestNameKey),
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
		return nil, errors.Wrapf(err, "getting metrics needing change point detection")
	}
	defer cur.Close(ctx)
	var res []PerformanceResultSeriesID
	err = cur.All(ctx, &res)
	if err != nil {
		return nil, errors.Wrapf(err, "decoding metrics needing change results")
	}
	return res, nil
}

// GetPerformanceResultSeriesIDs finds all performance result series IDs in the
// the database.
func GetPerformanceResultSeriesIDs(ctx context.Context, env cedar.Environment) ([]PerformanceResultSeriesID, error) {
	cur, err := env.GetDB().Collection(perfResultCollection).Aggregate(ctx, []bson.M{
		{
			"$match": bson.M{
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoProjectKey):  bson.M{"$exists": true},
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVariantKey):  bson.M{"$exists": true},
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoOrderKey):    bson.M{"$exists": true},
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskNameKey): bson.M{"$exists": true},
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTestNameKey): bson.M{"$exists": true},
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoMainlineKey): true,
			},
		},
		{
			"$unwind": "$" + bsonutil.GetDottedKeyName(perfRollupsKey, perfRollupsStatsKey),
		},
		{
			"$group": bson.M{
				"_id": bson.M{
					"project": "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoProjectKey),
					"variant": "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVariantKey),
					"task":    "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskNameKey),
					"test":    "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTestNameKey),
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
		return nil, errors.Wrap(err, "aggregating time series ids")
	}
	defer cur.Close(ctx)
	var res []PerformanceResultSeriesID
	err = cur.All(ctx, &res)
	if err != nil {
		return nil, errors.Wrap(err, "decoding time series ids")
	}
	return res, nil
}

// GetPerformanceData queries the database to get the performance result time
// series data associated with the given series ID.
func GetPerformanceData(ctx context.Context, env cedar.Environment, performanceResultId PerformanceResultSeriesID) ([]PerformanceData, error) {
	pipe := []bson.M{
		{
			"$match": bson.M{
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoProjectKey):  performanceResultId.Project,
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVariantKey):  performanceResultId.Variant,
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoOrderKey):    bson.M{"$exists": true},
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskNameKey): performanceResultId.Task,
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTestNameKey): performanceResultId.Test,
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoMainlineKey): true,
			},
		},
		{
			"$unwind": "$" + bsonutil.GetDottedKeyName(perfRollupsKey, perfRollupsStatsKey),
		},
		{
			"$group": bson.M{
				"_id": bson.M{
					"project":     "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoProjectKey),
					"variant":     "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVariantKey),
					"task":        "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskNameKey),
					"test":        "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTestNameKey),
					"measurement": "$" + bsonutil.GetDottedKeyName(perfRollupsKey, perfRollupsStatsKey, perfRollupValueNameKey),
					"args":        "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoArgumentsKey),
				},
				"time_series": bson.M{
					"$push": bson.M{
						"value": bson.M{
							"$ifNull": bson.A{
								"$" + bsonutil.GetDottedKeyName(perfRollupsKey, perfRollupsStatsKey, perfRollupValueValueKey),
								0,
							},
						},
						"order":          "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoOrderKey),
						"perf_result_id": "$_id",
						"version":        "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVersionKey),
					},
				},
			},
		},
	}
	cur, err := env.GetDB().Collection(perfResultCollection).Aggregate(ctx, pipe)
	if err != nil {
		return nil, errors.Wrap(err, "aggregating time series")
	}
	defer cur.Close(ctx)
	var res []PerformanceData
	err = cur.All(ctx, &res)
	if err != nil {
		return nil, errors.Wrap(err, "decoding time series")
	}
	if len(res) == 0 {
		return nil, nil
	}
	return res, nil
}
