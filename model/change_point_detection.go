package model

import (
	"context"
	"math"
	"time"

	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

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
	Index        int           `bson:"index" json:"index" yaml:"index"`
	Measurement  string        `bson:"measurement" json:"measurement" yaml:"measurement"`
	CalculatedOn time.Time     `bson:"calculated_on" json:"calculated_on" yaml:"calculated_on"`
	Algorithm    AlgorithmInfo `bson:"algorithm" json:"algorithm" yaml:"algorithm"`
	Triage       TriageInfo    `bson:"triage" json:"triage" yaml:"triage"`
}

func CreateChangePoint(index int, measurement string, algorithmName string, algorithmVersion int, algoOptions []AlgorithmOption) ChangePoint {
	cp := ChangePoint{
		Index: index,
		Algorithm: AlgorithmInfo{
			Name:    algorithmName,
			Version: algorithmVersion,
			Options: algoOptions,
		},
		CalculatedOn: time.Now(),
		Measurement:  measurement,
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

func TriageStatuses() []TriageStatus {
	return []TriageStatus{TriageStatusUntriaged, TriageStatusTruePositive, TriageStatusFalsePositive, TriageStatusUnderInvestigation}
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
	Measurement  string            `bson:"measurement"`
	TimeSeries   []TimeSeriesEntry `bson:"time_series"`
	ChangePoints []ChangePoint     `bson:"change_points"`
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
		//Filter out any change points unrelated to this rollup
		{
			"$addFields": bson.M{
				bsonutil.GetDottedKeyName(perfAnalysisKey, perfAnalysisChangePointsKey): bson.M{
					"$filter": bson.M{
						"input": "$" + bsonutil.GetDottedKeyName(perfAnalysisKey, perfAnalysisChangePointsKey),
						"as":    "cp",
						"cond": bson.M{
							"$eq": bson.A{"$$cp.measurement", "$" + bsonutil.GetDottedKeyName(perfRollupsKey, perfRollupsStatsKey, perfRollupValueNameKey)},
						},
					},
				},
			},
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
				"change_points": bson.M{
					"$push": "$" + bsonutil.GetDottedKeyName(perfAnalysisKey, perfAnalysisChangePointsKey),
				},
			},
		},
		// Flatter the change points into one array
		{
			"$addFields": bson.M{
				"change_points": bson.M{
					"$reduce": bson.M{
						"input":        "$change_points",
						"initialValue": bson.A{},
						"in":           bson.M{"$concatArrays": bson.A{"$$value", "$$this"}},
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
						"measurement":   "$_id.measurement",
						"time_series":   "$time_series",
						"change_points": "$change_points",
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

func GetTotalPagesForChangePointsGroupedByVersion(ctx context.Context, env cedar.Environment, projectId string, pageSize int) (int, error) {
	countKey := "count"
	pipe := appendAfterBaseGetChangePointsByVersionAgg(projectId, bson.M{
		"$count": countKey,
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
		return int(math.Ceil(float64(res.Count) / float64(pageSize))), nil
	}
	return 0, errors.New("Not able to get count of total changepoints matching query")
}

func appendAfterBaseGetChangePointsByVersionAgg(projectId string, additionalSteps ...bson.M) []bson.M {
	return append([]bson.M{
		{
			"$match": bson.M{
				bsonutil.GetDottedKeyName(perfInfoKey, perfResultInfoProjectKey):             projectId,
				bsonutil.GetDottedKeyName(perfAnalysisKey, perfAnalysisChangePointsKey, "0"): bson.M{"$exists": true},
			},
		},
		{
			"$group": bson.M{
				"_id":          "$info.version",
				"perf_results": bson.M{"$push": "$$ROOT"},
				"order":        bson.M{"$first": "$info.order"},
			},
		},
	}, additionalSteps...)
}

func GetChangePointsGroupedByVersion(ctx context.Context, env cedar.Environment, projectId string, page, pageSize int) ([]GetChangePointsGroupedByVersionResult, error) {
	pipe := appendAfterBaseGetChangePointsByVersionAgg(projectId, []bson.M{
		{
			"$sort": bson.M{
				"order": -1,
			},
		},
		{
			"$skip": page * pageSize,
		},
		{
			"$limit": pageSize,
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

func TriageChangePoints(ctx context.Context, env cedar.Environment, changePoints map[string]string, status TriageStatus) error {
	session, err := env.GetDB().Client().StartSession()
	if err != nil {
		return errors.Wrap(err, "Unable to start session for triage update")
	}
	defer session.EndSession(ctx)
	if err := session.StartTransaction(); err != nil {
		return errors.Wrap(err, "Unable to start transaction for triage update")
	}
	err = mongo.WithSession(ctx, session, func(sc mongo.SessionContext) error {
		for perfResultId, measurement := range changePoints {
			filter := bson.M{
				perfIDKey: perfResultId,
				bsonutil.GetDottedKeyName(perfAnalysisKey, perfAnalysisChangePointsKey, perfChangePointMeasurementKey): measurement,
			}
			update := bson.M{
				"$set": bson.M{
					bsonutil.GetDottedKeyName(perfAnalysisKey, perfAnalysisChangePointsKey, "$", perfChangePointTriageKey, perfTriageInfoStatusKey):    status,
					bsonutil.GetDottedKeyName(perfAnalysisKey, perfAnalysisChangePointsKey, "$", perfChangePointTriageKey, perfTriageInfoTriagedOnKey): time.Now(),
				},
			}
			_, err := env.GetDB().Collection(perfResultCollection).UpdateOne(ctx, filter, update)
			if err != nil {
				err2 := session.AbortTransaction(ctx)
				if err2 != nil {
					return errors.Wrap(err, "Failed to abort transaction during failed change point triage")
				}
				return errors.Wrap(err, "Unable to change triage status of change point")
			}
		}
		err := session.CommitTransaction(sc)
		if err != nil {
			return errors.Wrap(err, "Failed to commit transaction during change point triage")
		}
		return nil
	})
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
		return errors.Errorf("Error triaging change point on measurement %s for performance results %s, modified count: %d", measurement, perfResultID, res.ModifiedCount)
	}
	return nil
}