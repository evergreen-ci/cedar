package model

import (
	"context"
	"math"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
	Index         int           `bson:"index" json:"index" yaml:"index"`
	Measurement   string        `bson:"measurement" json:"measurement" yaml:"measurement"`
	CalculatedOn  time.Time     `bson:"calculated_on" json:"calculated_on" yaml:"calculated_on"`
	Algorithm     AlgorithmInfo `bson:"algorithm" json:"algorithm" yaml:"algorithm"`
	Triage        TriageInfo    `bson:"triage" json:"triage" yaml:"triage"`
	PercentChange float64       `bson:"percent_change" json:"percent_change" yaml:"percent_change"`
}

func CreateChangePoint(index int, measurement string, algorithmName string, algorithmVersion int, algoOptions []AlgorithmOption, percentChange float64) ChangePoint {
	cp := ChangePoint{
		Index: index,
		Algorithm: AlgorithmInfo{
			Name:    algorithmName,
			Version: algorithmVersion,
			Options: algoOptions,
		},
		PercentChange: percentChange,
		CalculatedOn:  time.Now(),
		Measurement:   measurement,
		Triage: TriageInfo{
			Status: TriageStatusUntriaged,
		},
	}
	return cp
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
	TriageStatusUntriaged          TriageStatus = "untriaged"
	TriageStatusTruePositive       TriageStatus = "true_positive"
	TriageStatusFalsePositive      TriageStatus = "false_positive"
	TriageStatusUnderInvestigation TriageStatus = "under_investigation"
)

func (ts TriageStatus) Validate() error {
	switch ts {
	case TriageStatusUntriaged, TriageStatusTruePositive, TriageStatusFalsePositive, TriageStatusUnderInvestigation:
		return nil
	default:
		return errors.New("invalid triage status")
	}
}

type PerformanceResultSeriesID struct {
	Project   string           `bson:"project"`
	Variant   string           `bson:"variant"`
	Task      string           `bson:"task"`
	Test      string           `bson:"test"`
	Arguments map[string]int32 `bson:"args"`
}

type TimeSeriesEntry struct {
	PerfResultID string  `bson:"perf_result_id"`
	Value        float64 `bson:"value"`
	Order        int     `bson:"order"`
	Version      string  `bson:"version"`
}

type MeasurementData struct {
	Measurement string            `bson:"measurement"`
	TimeSeries  []TimeSeriesEntry `bson:"time_series"`
}

type PerformanceData struct {
	PerformanceResultId PerformanceResultSeriesID `bson:"_id"`
	Data                []MeasurementData         `bson:"data"`
}

type GetChangePointsGroupedByVersionResult struct {
	VersionID   string              `bson:"_id" json:"version_id"`
	PerfResults []PerformanceResult `bson:"perf_results" json:"perf_results"`
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
					"args":    "$" + bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoArgumentsKey),
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
		{
			"$group": bson.M{
				"_id": bson.M{
					"project": "$_id.project",
					"variant": "$_id.variant",
					"task":    "$_id.task",
					"test":    "$_id.test",
					"args":    "$_id.args",
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
	err := clearUntriagedChangePoints(ctx, env, performanceData.PerformanceResultId)
	if err != nil {
		return errors.Wrapf(err, "Unable to clear change points for measurement %s", performanceData.PerformanceResultId)
	}
	catcher := grip.NewBasicCatcher()
	for _, measurementData := range performanceData.Data {
		changePoints := mappedChangePoints[measurementData.Measurement]
		for _, cp := range changePoints {
			perfResultId := measurementData.TimeSeries[cp.Index].PerfResultID
			err = createChangePoint(ctx, env, perfResultId, cp)
			if err != nil {
				catcher.Add(errors.Wrapf(err, "Failed to update performance result with change point %s", perfResultId))
			}
		}
	}
	return catcher.Resolve()
}

type GetChangePointsGroupedByVersionOptions struct {
	ProjectID        string
	Page             int
	PageSize         int
	VariantRegex     string
	VersionRegex     string
	TaskRegex        string
	TestRegex        string
	MeasurementRegex string
	Arguments        map[string][]int
}

func GetTotalPagesForChangePointsGroupedByVersion(ctx context.Context, env cedar.Environment, args GetChangePointsGroupedByVersionOptions) (int, error) {
	pipe := appendAfterBaseGetChangePointsByVersionAgg(args, bson.M{
		"$count": "count",
	})
	cur, err := env.GetDB().Collection(perfResultCollection).Aggregate(ctx, pipe)
	if err != nil {
		return 0, errors.Wrap(err, "Cannot aggregate to get count of change points grouped by version")
	}
	defer cur.Close(ctx)
	res := struct {
		Count int `bson:"count"`
	}{}
	if cur.Next(ctx) {
		err = cur.Decode(&res)
		if err != nil {
			return 0, errors.Wrap(err, "Cannot decode response of getting count of change points grouped by version")
		}
		return int(math.Ceil(float64(res.Count) / float64(args.PageSize))), nil
	}
	return 0, nil
}

func appendAfterBaseGetChangePointsByVersionAgg(args GetChangePointsGroupedByVersionOptions, additionalSteps ...bson.M) []bson.M {
	matchStage := bson.M{
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoProjectKey):  args.ProjectID,
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVariantKey):  bson.M{"$regex": args.VariantRegex},
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVersionKey):  bson.M{"$regex": args.VersionRegex},
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskNameKey): bson.M{"$regex": args.TaskRegex},
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTestNameKey): bson.M{"$regex": args.TestRegex},
	}

	for arg, val := range args.Arguments {
		matchStage[bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoArgumentsKey, arg)] = bson.M{
			"$in": val,
		}
	}

	return append([]bson.M{
		//Filter based on perf_result level properties
		{
			"$match": matchStage,
		},
		//Filter out cps that don't match regex
		{
			"$addFields": bson.M{
				bsonutil.GetDottedKeyName(perfAnalysisKey, perfAnalysisChangePointsKey): bson.M{
					"$filter": bson.M{
						"input": "$" + bsonutil.GetDottedKeyName(perfAnalysisKey, perfAnalysisChangePointsKey),
						"as":    "cp",
						"cond":  bson.M{"$regexMatch": bson.M{"input": "$$cp.measurement", "regex": args.MeasurementRegex}},
					},
				},
			},
		},
		//Filter out perf results with no valid regex
		{
			"$match": bson.M{
				bsonutil.GetDottedKeyName(perfAnalysisKey, perfAnalysisChangePointsKey, "0"): bson.M{"$exists": true},
			},
		},
		{
			"$group": bson.M{
				"_id":          "$info.version",
				"perf_results": bson.M{"$push": "$_id"},
				"order":        bson.M{"$first": "$info.order"},
			},
		},
	}, additionalSteps...)
}

func GetChangePointsGroupedByVersion(ctx context.Context, env cedar.Environment, args GetChangePointsGroupedByVersionOptions) ([]GetChangePointsGroupedByVersionResult, error) {
	pipe := appendAfterBaseGetChangePointsByVersionAgg(args, []bson.M{
		{
			"$sort": bson.M{
				"order": -1,
			},
		},
		{
			"$skip": args.Page * args.PageSize,
		},
		{
			"$limit": args.PageSize,
		},
		{
			"$lookup": bson.M{
				"from":         perfResultCollection,
				"localField":   "perf_results",
				"foreignField": "_id",
				"as":           "perf_results",
			},
		},
	}...)
	cur, err := env.GetDB().Collection(perfResultCollection).Aggregate(ctx, pipe)
	if err != nil {
		return nil, errors.Wrap(err, "Cannot aggregate to get change points grouped by version")
	}
	defer cur.Close(ctx)
	var res []GetChangePointsGroupedByVersionResult
	err = cur.All(ctx, &res)
	if err != nil {
		return nil, errors.Wrap(err, "Cannot decode response of getting change points grouped by version")
	}
	return res, nil
}

func clearUntriagedChangePoints(ctx context.Context, env cedar.Environment, performanceResultId PerformanceResultSeriesID) error {
	seriesFilter := bson.M{
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoProjectKey):  performanceResultId.Project,
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoVariantKey):  performanceResultId.Variant,
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTaskNameKey): performanceResultId.Task,
		bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoTestNameKey): performanceResultId.Test,
	}
	clearingUntriagedUpdate := bson.M{
		"$pull": bson.M{
			bsonutil.GetDottedKeyName(perfAnalysisKey, perfAnalysisChangePointsKey): bson.M{
				perfChangePointMeasurementKey: TriageStatusUntriaged,
			},
		},
	}
	_, err := env.GetDB().Collection(perfResultCollection).UpdateMany(ctx, seriesFilter, clearingUntriagedUpdate)
	return errors.Wrap(err, "Unable to clear change points")
}

func createChangePoint(ctx context.Context, env cedar.Environment, resultToUpdate string, cp ChangePoint) error {
	filter := bson.M{"_id": resultToUpdate}
	update := bson.M{
		"$push": bson.M{
			bsonutil.GetDottedKeyName(perfAnalysisKey, perfAnalysisChangePointsKey): cp,
		},
	}
	_, err := env.GetDB().Collection(perfResultCollection).UpdateOne(ctx, filter, update)
	return errors.Wrap(err, "Unable to create change point")
}

type ChangePointInfo struct {
	PerfResultID string `json:"perf_result_id"`
	Measurement  string `json:"measurement"`
}

func TriageChangePoints(ctx context.Context, env cedar.Environment, changePoints []ChangePointInfo, status TriageStatus) error {
	coll := env.GetDB().Collection(perfResultCollection)

	conditions := make([]bson.M, len(changePoints))
	for i, stub := range changePoints {
		conditions[i] = bson.M{
			perfIDKey: stub.PerfResultID,
			bsonutil.GetDottedKeyName(perfAnalysisKey, perfAnalysisChangePointsKey, perfChangePointMeasurementKey): stub.Measurement,
		}
	}
	filter := bson.M{
		"$or": conditions,
	}

	cur, err := coll.Find(ctx, filter)
	if err != nil {
		return errors.Wrap(err, "problem executing query finding change points for triage")
	}
	var results []PerformanceResult
	if err := cur.All(ctx, &results); err != nil {
		return errors.Wrap(err, "problem decoding performance results for triage")
	}

ChangePointsLoop:
	for _, stub := range changePoints {
		for _, res := range results {
			if res.ID == stub.PerfResultID {
				for _, cp := range res.Analysis.ChangePoints {
					if cp.Measurement == stub.Measurement {
						continue ChangePointsLoop
					}
				}
			}
		}
		return errors.Errorf("problem finding change point <%s> for performance result %s", stub.Measurement, stub.PerfResultID)
	}

	update := bson.M{
		"$set": bson.M{
			bsonutil.GetDottedKeyName(perfAnalysisKey, perfAnalysisChangePointsKey, "$", perfChangePointTriageKey, perfTriageInfoStatusKey):    status,
			bsonutil.GetDottedKeyName(perfAnalysisKey, perfAnalysisChangePointsKey, "$", perfChangePointTriageKey, perfTriageInfoTriagedOnKey): time.Now(),
		},
	}
	var operations []mongo.WriteModel
	for _, cond := range conditions {
		operations = append(operations, &mongo.UpdateOneModel{Filter: cond, Update: update})
	}

	if _, err := env.GetDB().Collection(perfResultCollection).BulkWrite(ctx, operations, options.BulkWrite().SetOrdered(true)); err != nil {
		return errors.Wrap(err, "problem performing triaging update")
	}
	return nil
}
func TriageChangePoint(ctx context.Context, env cedar.Environment, perfResultID string, measurement string, status TriageStatus) error {
	filter := bson.M{
		perfIDKey: perfResultID,
		bsonutil.GetDottedKeyName(perfAnalysisKey, perfAnalysisChangePointsKey, perfChangePointMeasurementKey): measurement,
	}
	update := bson.M{
		"$set": bson.M{
			bsonutil.GetDottedKeyName(perfAnalysisKey, perfAnalysisChangePointsKey, "$", perfChangePointTriageKey, perfTriageInfoStatusKey):    status,
			bsonutil.GetDottedKeyName(perfAnalysisKey, perfAnalysisChangePointsKey, "$", perfChangePointTriageKey, perfTriageInfoTriagedOnKey): time.Now(),
		},
	}
	res, err := env.GetDB().Collection(perfResultCollection).UpdateOne(ctx, filter, update)
	if err != nil {
		return errors.Wrap(err, "Unable to change triage status of change point")
	}
	if res.ModifiedCount != 1 {
		return errors.Errorf("problem triaging change point on measurement %s for performance results %s, modified count: %d", measurement, perfResultID, res.ModifiedCount)
	}
	return nil
}
