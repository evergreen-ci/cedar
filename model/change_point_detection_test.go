package model

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func setup() {
	dbName := "test_cedar_change_point_detection"
	env, err := cedar.NewEnvironment(context.Background(), dbName, &cedar.Configuration{
		MongoDBURI:    "mongodb://localhost:27017",
		DatabaseName:  dbName,
		SocketTimeout: time.Minute,
		NumWorkers:    2,
	})
	if err != nil {
		panic(err)
	}
	cedar.SetEnvironment(env)
}

func tearDown(env cedar.Environment) error {
	conf, session, err := cedar.GetSessionWithConfig(env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()
	return errors.WithStack(session.DB(conf.DatabaseName).DropDatabase())
}

func provisionDB(ctx context.Context, env cedar.Environment) []string {
	var rollups []struct {
		info    PerformanceResultInfo
		rollups []PerfRollupValue
	}
	for i := 0; i < 10; i++ {
		newRollup := struct {
			info    PerformanceResultInfo
			rollups []PerfRollupValue
		}{
			info: PerformanceResultInfo{
				Project:  "project",
				Variant:  "variant",
				Version:  "version" + strconv.Itoa(i+1),
				Order:    i + 1,
				TestName: "test",
				TaskName: "task",
				Mainline: true,
			},
			rollups: []PerfRollupValue{
				{
					Name:       "measurement",
					MetricType: MetricTypeSum,
					Version:    0,
					Value:      i,
				},
				{
					Name:       "measurement_another",
					MetricType: MetricTypeSum,
					Version:    0,
					Value:      i,
				},
			},
		}
		rollups = append(rollups, newRollup)
	}
	var ids []string

	for idx, result := range rollups {
		performanceResult := CreatePerformanceResult(result.info, nil, result.rollups)
		performanceResult.CreatedAt = time.Now().Add(time.Second * -1)
		performanceResult.Setup(env)
		err := performanceResult.SaveNew(ctx)
		if err != nil {
			panic(err)
		}
		err = createChangePoint(ctx, env, performanceResult.ID, ChangePoint{
			Index:        idx,
			Measurement:  "measurement",
			CalculatedOn: time.Now(),
			Triage:       TriageInfo{Status: TriageStatusUntriaged},
		})
		if err != nil {
			panic(err)
		}
		ids = append(ids, performanceResult.ID)
	}
	return ids
}

func TestTriageChangePointsJob(t *testing.T) {
	setup()
	env := cedar.GetEnvironment()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, tearDown(env))
	}()

	t.Run("TriagesSuccessfully", func(t *testing.T) {
		_ = env.GetDB().Drop(ctx)
		ids := provisionDB(ctx, env)

		var cps []ChangePointStub
		for _, id := range ids[:5] {
			cps = append(cps, ChangePointStub{
				PerfResultID: id,
				Measurement:  "measurement",
			})
		}
		require.NoError(t, TriageChangePoints(ctx, env, cps, TriageStatusTruePositive))

		filter := bson.M{
			"analysis.change_points": bson.M{"$ne": []struct{}{}},
		}
		res, err := env.GetDB().Collection("perf_results").Find(ctx, filter)
		require.NoError(t, err)
		var result []PerformanceResult
		assert.NoError(t, res.All(ctx, &result))

		require.Len(t, result, 10)

		for idx, res := range result {
			if idx < 5 {
				require.Equal(t, res.Analysis.ChangePoints[0].Triage.Status, TriageStatusTruePositive)
			} else {
				require.Equal(t, res.Analysis.ChangePoints[0].Triage.Status, TriageStatusUntriaged)
			}
		}
	})

	t.Run("RollsbackOnError", func(t *testing.T) {
		_ = env.GetDB().Drop(ctx)
		ids := provisionDB(ctx, env)

		var cps []ChangePointStub
		for _, id := range ids[:5] {
			cps = append(cps, ChangePointStub{
				PerfResultID: id,
				Measurement:  "measurement",
			})
		}
		// We're gonna miss on one, everything should roll back
		cps = append(cps, ChangePointStub{
			PerfResultID: "wrong_id",
			Measurement:  "measurement",
		})
		assert.NotNil(t, TriageChangePoints(ctx, env, cps, TriageStatusTruePositive))

		filter := bson.M{
			"analysis.change_points": bson.M{"$ne": []struct{}{}},
		}
		res, err := env.GetDB().Collection("perf_results").Find(ctx, filter)
		require.NoError(t, err)
		var result []PerformanceResult
		assert.NoError(t, res.All(ctx, &result))

		require.Len(t, result, 10)

		for _, res := range result {
			assert.Equal(t, res.Analysis.ChangePoints[0].Triage.Status, TriageStatusUntriaged)
		}
	})
}
