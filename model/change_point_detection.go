package model

import (
	"context"
	"time"

	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/evergreen-ci/cedar"
)

type PerfAnalysis struct {
	ChangePoints []ChangePoint `bson:"change_points" json:"change_points" yaml:"change_points"`
	ProcessedAt  time.Time     `bson:"processed_at" json:"processed_at" yaml:"processed_at"`
}

var (
	perfAnalysisChangePointsKey = bsonutil.MustHaveTag(PerfAnalysis{}, "ChangePoints")
	perfAnalysisProcessedAtKey  = bsonutil.MustHaveTag(PerfAnalysis{}, "ProcessedAt")
)

type ChangePoint struct {
	Index        int
	Measurement  string        `bson:"measurement" json:"measurement" yaml:"measurement"`
	CalculatedOn time.Time     `bson:"calculated_on" json:"calculated_on" yaml:"calculated_on"`
	Algorithm    AlgorithmInfo `bson:"algorithm" json:"algorithm" yaml:"algorithm"`
	Triage       TriageInfo    `bson:"triage" json:"triage" yaml:"triage"`
}

var (
	perfChangePointMeasurementKey  = bsonutil.MustHaveTag(ChangePoint{}, "Measurement")
	perfChangePointCalculatedOnKey = bsonutil.MustHaveTag(ChangePoint{}, "CalculatedOn")
	perfChangePointAlgorithmKey    = bsonutil.MustHaveTag(ChangePoint{}, "Algorithm")
	perfChangePointTriageKey       = bsonutil.MustHaveTag(ChangePoint{}, "Triage")
)

type AlgorithmInfo struct {
	Name    string            `bson:"name" json:"name" yaml:"name"`
	Version int               `bson:"version" json:"version" yaml:"version"`
	Options []AlgorithmOption `bson:"options" json:"options" yaml:"options"`
}

var (
	perfAlgorithmNameKey    = bsonutil.MustHaveTag(AlgorithmInfo{}, "Name")
	perfAlgorithmVersionKey = bsonutil.MustHaveTag(AlgorithmInfo{}, "Version")
	perfAlgorithmOptionsKey = bsonutil.MustHaveTag(AlgorithmInfo{}, "Options")
)

type AlgorithmOption struct {
	Name  string      `bson:"name" json:"name" yaml:"name"`
	Value interface{} `bson:"value" json:"value" yaml:"value"`
}

var (
	perfAlgorithmOptionNameKey  = bsonutil.MustHaveTag(AlgorithmOption{}, "Name")
	perfAlgorithmOptionValueKey = bsonutil.MustHaveTag(AlgorithmOption{}, "Value")
)

type TriageInfo struct {
	TriagedOn time.Time    `bson:"triaged_on" json:"triaged_on" yaml:"triaged_on"`
	Status    TriageStatus `bson:"triage_status" json:"triage_status" yaml:"triage_status"`
}

var (
	perfTriageInfoTriagedOnKey = bsonutil.MustHaveTag(TriageInfo{}, "TriagedOn")
	perfTriageInfoStatusKey    = bsonutil.MustHaveTag(TriageInfo{}, "Status")
)

type TriageStatus string

const (
	Untriaged          TriageStatus = "untriaged"
	TruePositive       TriageStatus = "true_positive"
	FalsePositive      TriageStatus = "false_positive"
	UnderInvestigation TriageStatus = "under_investigation"
)

func (ts TriageStatus) Validate() error {
	switch ts {
	case Untriaged, TruePositive, FalsePositive, UnderInvestigation:
		return nil
	default:
		return errors.New("invalid triage status")
	}
}

type PerformanceResultSeriesID struct {
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
	PerformanceResultId PerformanceResultSeriesID `bson:"_id"`
	Data                []MeasurementData         `bson:"data"`
}

func MarkPerformanceResultsAsAnalyzed(ctx context.Context, env cedar.Environment, performanceResultId PerformanceResultSeriesID) error {
	filter := bson.M{
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoProjectKey):  performanceResultId.Project,
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVariantKey):  performanceResultId.Variant,
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskNameKey): performanceResultId.Task,
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTestNameKey): performanceResultId.Test,
	}

	update := bson.M{
		"$currentDate": bson.M{
			bsonutil.GetDottedKeyName(perfAnalysisKey, perfAnalysisProcessedAtKey): true,
		},
	}
	_, err := env.GetDB().Collection(perfResultCollection).UpdateMany(ctx, filter, update)
	if err != nil {
		return errors.Wrapf(err, "Unable to mark performance results as analyzed for change points")
	}
	return nil
}

func GetPerformanceResultSeriesIdsNeedingChangePointDetection(ctx context.Context, env cedar.Environment) ([]PerformanceResultSeriesID, error) {
	cur, err := env.GetDB().Collection(perfResultCollection).Aggregate(ctx, []bson.M{
		{
			"$match": bson.M{
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoOrderKey):    bson.M{"$exists": true},
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoMainlineKey): true,
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoProjectKey):  bson.M{"$exists": true},
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVariantKey):  bson.M{"$exists": true},
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskNameKey): bson.M{"$exists": true},
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTestNameKey): bson.M{"$exists": true},
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
		return nil, errors.Wrapf(err, "Unable to get metrics needing change point detection")
	}
	defer cur.Close(ctx)
	var res []PerformanceResultSeriesID
	err = cur.All(ctx, &res)
	if err != nil {
		return nil, errors.Wrapf(err, "Unable to decode metrics needing change results")
	}
	return res, nil
}

func GetPerformanceResultSeriesIDs(ctx context.Context, env cedar.Environment) ([]PerformanceResultSeriesID, error) {
	cur, err := env.GetDB().Collection(perfResultCollection).Aggregate(ctx, []bson.M{
		{
			"$match": bson.M{
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoOrderKey):    bson.M{"$exists": true},
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoMainlineKey): true,
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoProjectKey):  bson.M{"$exists": true},
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVariantKey):  bson.M{"$exists": true},
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskNameKey): bson.M{"$exists": true},
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTestNameKey): bson.M{"$exists": true},
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
		return nil, errors.Wrap(err, "Cannot aggregate time series ids")
	}
	defer cur.Close(ctx)
	var res []PerformanceResultSeriesID
	err = cur.All(ctx, &res)
	if err != nil {
		return nil, errors.Wrap(err, "Could not decode time series ids")
	}
	return res, nil
}
func GetPerformanceData(ctx context.Context, env cedar.Environment, performanceResultId PerformanceResultSeriesID) (*PerformanceData, error) {
	pipe := []bson.M{
		{
			"$match": bson.M{
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoOrderKey):    bson.M{"$exists": true},
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoMainlineKey): true,
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoProjectKey):  performanceResultId.Project,
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVariantKey):  performanceResultId.Variant,
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskNameKey): performanceResultId.Task,
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTestNameKey): performanceResultId.Test,
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
				"data": bson.M{
					"$push": bson.M{
						"measurement": "$_id.measurement",
						"time_series": "$time_series",
					},
				},
			},
		},
	}
	cur, err := env.GetDB().Collection(perfResultCollection).Aggregate(ctx, pipe)
	if err != nil {
		return nil, errors.Wrap(err, "Cannot aggregate time series")
	}
	defer cur.Close(ctx)
	var res []PerformanceData
	err = cur.All(ctx, &res)
	if err != nil {
		return nil, errors.Wrap(err, "Could not decode time series")
	}
	if len(res) < 1 {
		return nil, nil
	}
	return &res[0], nil
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

func clearChangePoints(ctx context.Context, env cedar.Environment, performanceResultId PerformanceResultSeriesID) error {
	seriesFilter := bson.M{
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoProjectKey):  performanceResultId.Project,
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVariantKey):  performanceResultId.Variant,
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskNameKey): performanceResultId.Task,
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTestNameKey): performanceResultId.Test,
	}
	clearingUpdate := bson.M{
		"$set": bson.M{
			bsonutil.GetDottedKeyName(perfAnalysisKey, perfAnalysisChangePointsKey): []bson.M{},
		},
	}
	_, err := env.GetDB().Collection(perfResultCollection).UpdateMany(ctx, seriesFilter, clearingUpdate)
	return errors.Wrap(err, "Unable to clear change points")
}

func createChangePoint(ctx context.Context, env cedar.Environment, resultToUpdate string, measurement string, algorithm AlgorithmInfo) error {
	filter := bson.M{"_id": resultToUpdate}
	update := bson.M{
		"$push": bson.M{
			bsonutil.GetDottedKeyName(perfAnalysisKey, perfAnalysisChangePointsKey): ChangePoint{
				Measurement:  measurement,
				Algorithm:    algorithm,
				CalculatedOn: time.Now(),
			},
		},
	}
	_, err := env.GetDB().Collection(perfResultCollection).UpdateOne(ctx, filter, update)
	return errors.Wrap(err, "Unable to create change point")
}
