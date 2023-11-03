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
	return fmt.Sprintf("%s %s %s %s %s", p.Project, p.Variant, p.Task, p.Test, p.Arguments)
}

// MarkPerformanceResultsAsAnalyzed marks the most recent mainline performance
// results with the given series ID as analyzed with the current date
// timestamp.
func MarkPerformanceResultsAsAnalyzed(ctx context.Context, env cedar.Environment, performanceResultId PerformanceResultSeriesID) error {
	filter := bson.M{
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoProjectKey):   performanceResultId.Project,
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVariantKey):   performanceResultId.Variant,
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskNameKey):  performanceResultId.Task,
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTestNameKey):  performanceResultId.Test,
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoArgumentsKey): performanceResultId.Arguments,
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoMainlineKey):  true,
		perfCreatedAtKey: bson.M{"$gt": time.Now().Add(-7 * 24 * time.Hour)},
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

// UnanalyzedPerformanceSeries represents a set of series with the same
// project/variant/task/test/args combination and varying measurements that
// need to be analyzed by the signal processing service.
type UnanalyzedPerformanceSeries struct {
	Project      string               `bson:"project"`
	Variant      string               `bson:"variant"`
	Task         string               `bson:"task"`
	Test         string               `bson:"test"`
	Measurements []string             `bson:"measurements"`
	Arguments    PerformanceArguments `bson:"args"`
}

var (
	unanalyzedPerformanceSeriesProjectKey      = bsonutil.MustHaveTag(UnanalyzedPerformanceSeries{}, "Project")
	unanalyzedPerformanceSeriesVariantKey      = bsonutil.MustHaveTag(UnanalyzedPerformanceSeries{}, "Variant")
	unanalyzedPerformanceSeriesTaskKey         = bsonutil.MustHaveTag(UnanalyzedPerformanceSeries{}, "Task")
	unanalyzedPerformanceSeriesTestKey         = bsonutil.MustHaveTag(UnanalyzedPerformanceSeries{}, "Test")
	unanalyzedPerformanceSeriesMeasurementsKey = bsonutil.MustHaveTag(UnanalyzedPerformanceSeries{}, "Measurements")
	unanalyzedPerformanceSeriesArgumentsKey    = bsonutil.MustHaveTag(UnanalyzedPerformanceSeries{}, "Arguments")
)

// CreateBaseSeriesID returns a PerformanceSeriesID without a measurement.
func (s UnanalyzedPerformanceSeries) CreateBaseSeriesID() PerformanceResultSeriesID {
	return PerformanceResultSeriesID{
		Project:   s.Project,
		Variant:   s.Variant,
		Task:      s.Task,
		Test:      s.Test,
		Arguments: s.Arguments,
	}
}

// CreateSeriesIDs unwinds the Measurements slice to create an individual
// PerformanceResultSeriesID for each measurement.
func (s UnanalyzedPerformanceSeries) CreateSeriesIDs() []PerformanceResultSeriesID {
	ids := make([]PerformanceResultSeriesID, len(s.Measurements))
	for i, measurement := range s.Measurements {
		ids[i] = PerformanceResultSeriesID{
			Project:     s.Project,
			Variant:     s.Variant,
			Task:        s.Task,
			Test:        s.Test,
			Measurement: measurement,
			Arguments:   s.Arguments,
		}
	}

	return ids
}

// GetUnanalyzedPerformanceSeries queries the DB and gets all the most recent
// mainline performance series that contain results that have not yet been
// analyzed.
func GetUnanalyzedPerformanceSeries(ctx context.Context, env cedar.Environment) ([]UnanalyzedPerformanceSeries, error) {
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
				perfCreatedAtKey: bson.M{"$gt": time.Now().Add(-7 * 24 * time.Hour)},
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
		// Limit the number of perf results we match against to avoid
		// long execution times.
		{
			"$limit": 1000,
		},
		{
			"$unwind": bson.M{
				"path": "$" + bsonutil.GetDottedKeyName(perfRollupsKey, perfRollupsStatsKey),
			},
		},
		{
			"$group": bson.M{
				"_id": bson.M{
					unanalyzedPerformanceSeriesProjectKey:   "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoProjectKey),
					unanalyzedPerformanceSeriesVariantKey:   "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVariantKey),
					unanalyzedPerformanceSeriesTaskKey:      "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskNameKey),
					unanalyzedPerformanceSeriesTestKey:      "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTestNameKey),
					unanalyzedPerformanceSeriesArgumentsKey: "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoArgumentsKey),
				},
				unanalyzedPerformanceSeriesMeasurementsKey: bson.M{
					"$addToSet": "$" + bsonutil.GetDottedKeyName(perfRollupsKey, perfRollupsStatsKey, perfRollupValueNameKey),
				},
			},
		},
		// In order to avoid overwhelming the Signal Processing Service
		// with requests, we need to limit the number of backlog
		// updates sent during periodic backfill jobs.
		{
			"$limit": 150,
		},
		{
			"$replaceRoot": bson.M{
				"newRoot": bson.M{
					unanalyzedPerformanceSeriesProjectKey:      "$" + bsonutil.GetDottedKeyName("_id", unanalyzedPerformanceSeriesProjectKey),
					unanalyzedPerformanceSeriesVariantKey:      "$" + bsonutil.GetDottedKeyName("_id", unanalyzedPerformanceSeriesVariantKey),
					unanalyzedPerformanceSeriesTaskKey:         "$" + bsonutil.GetDottedKeyName("_id", unanalyzedPerformanceSeriesTaskKey),
					unanalyzedPerformanceSeriesTestKey:         "$" + bsonutil.GetDottedKeyName("_id", unanalyzedPerformanceSeriesTestKey),
					unanalyzedPerformanceSeriesArgumentsKey:    "$" + bsonutil.GetDottedKeyName("_id", unanalyzedPerformanceSeriesArgumentsKey),
					unanalyzedPerformanceSeriesMeasurementsKey: "$" + unanalyzedPerformanceSeriesMeasurementsKey,
				},
			},
		},
	})
	if err != nil {
		return nil, errors.Wrapf(err, "getting metrics needing change point detection")
	}
	defer cur.Close(ctx)
	var res []UnanalyzedPerformanceSeries
	err = cur.All(ctx, &res)
	if err != nil {
		return nil, errors.Wrapf(err, "decoding metrics needing change results")
	}
	return res, nil
}

// GetAllPerformanceResultSeriesIDs finds all performance result series in the
// DB.
func GetAllPerformanceResultSeriesIDs(ctx context.Context, env cedar.Environment) ([]UnanalyzedPerformanceSeries, error) {
	cur, err := env.GetDB().Collection(perfResultCollection).Aggregate(ctx, []bson.M{
		{
			"$match": bson.M{
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoProjectKey):  bson.M{"$exists": true},
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVariantKey):  bson.M{"$exists": true},
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoOrderKey):    bson.M{"$exists": true},
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskNameKey): bson.M{"$exists": true},
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTestNameKey): bson.M{"$exists": true},
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoMainlineKey): true,
				bsonutil.GetDottedKeyName(perfRollupsKey, perfRollupsStatsKey):    bson.M{"$not": bson.M{"$size": 0}},
			},
		},
		{
			"$unwind": "$" + bsonutil.GetDottedKeyName(perfRollupsKey, perfRollupsStatsKey),
		},
		{
			"$group": bson.M{
				"_id": bson.M{
					unanalyzedPerformanceSeriesProjectKey:   "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoProjectKey),
					unanalyzedPerformanceSeriesVariantKey:   "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVariantKey),
					unanalyzedPerformanceSeriesTaskKey:      "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskNameKey),
					unanalyzedPerformanceSeriesTestKey:      "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTestNameKey),
					unanalyzedPerformanceSeriesArgumentsKey: "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoArgumentsKey),
				},
				unanalyzedPerformanceSeriesMeasurementsKey: bson.M{
					"$addToSet": "$" + bsonutil.GetDottedKeyName(perfRollupsKey, perfRollupsStatsKey, perfRollupValueNameKey),
				},
			},
		},
		{
			"$replaceRoot": bson.M{
				"newRoot": bson.M{
					unanalyzedPerformanceSeriesProjectKey:      "$" + bsonutil.GetDottedKeyName("_id", unanalyzedPerformanceSeriesProjectKey),
					unanalyzedPerformanceSeriesVariantKey:      "$" + bsonutil.GetDottedKeyName("_id", unanalyzedPerformanceSeriesVariantKey),
					unanalyzedPerformanceSeriesTaskKey:         "$" + bsonutil.GetDottedKeyName("_id", unanalyzedPerformanceSeriesTaskKey),
					unanalyzedPerformanceSeriesTestKey:         "$" + bsonutil.GetDottedKeyName("_id", unanalyzedPerformanceSeriesTestKey),
					unanalyzedPerformanceSeriesArgumentsKey:    "$" + bsonutil.GetDottedKeyName("_id", unanalyzedPerformanceSeriesArgumentsKey),
					unanalyzedPerformanceSeriesMeasurementsKey: "$" + unanalyzedPerformanceSeriesMeasurementsKey,
				},
			},
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "aggregating time series ids")
	}
	defer cur.Close(ctx)
	var res []UnanalyzedPerformanceSeries
	err = cur.All(ctx, &res)
	if err != nil {
		return nil, errors.Wrap(err, "decoding time series ids")
	}
	return res, nil
}
