package model

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/utility"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func TestCreateHistoricalTestData(t *testing.T) {
	info := HistoricalTestDataInfo{
		Project:     "project",
		Variant:     "variant",
		TaskName:    "task_name",
		TestName:    "test_name",
		RequestType: "request_type",
		Date:        time.Now(),
	}

	t.Run("MissingProject", func(t *testing.T) {
		project := info.Project
		info.Project = ""
		htd, err := CreateHistoricalTestData(info)
		assert.Nil(t, htd)
		assert.Error(t, err)
		info.Project = project
	})
	t.Run("MissingVariant", func(t *testing.T) {
		variant := info.Variant
		info.Variant = ""
		htd, err := CreateHistoricalTestData(info)
		assert.Nil(t, htd)
		assert.Error(t, err)
		info.Variant = variant
	})
	t.Run("MissingTaskName", func(t *testing.T) {
		taskName := info.TaskName
		info.TaskName = ""
		htd, err := CreateHistoricalTestData(info)
		assert.Nil(t, htd)
		assert.Error(t, err)
		info.TaskName = taskName
	})
	t.Run("MissingTestName", func(t *testing.T) {
		testName := info.TestName
		info.TestName = ""
		htd, err := CreateHistoricalTestData(info)
		assert.Nil(t, htd)
		assert.Error(t, err)
		info.TestName = testName
	})
	t.Run("MissingRequestType", func(t *testing.T) {
		RequestType := info.RequestType
		info.RequestType = ""
		htd, err := CreateHistoricalTestData(info)
		assert.Nil(t, htd)
		assert.Error(t, err)
		info.RequestType = RequestType
	})
	t.Run("MissingDate", func(t *testing.T) {
		date := info.Date
		info.Date = time.Time{}
		htd, err := CreateHistoricalTestData(info)
		assert.Nil(t, htd)
		assert.Error(t, err)
		info.Date = date
	})
	t.Run("ValidInfo", func(t *testing.T) {
		expectedDate := time.Date(
			info.Date.Year(),
			info.Date.Month(),
			info.Date.Day(),
			0, 0, 0, 0,
			time.UTC,
		)
		htd, err := CreateHistoricalTestData(info)
		assert.NoError(t, err)
		require.NotNil(t, htd)
		assert.Equal(t, info.Project, htd.Info.Project)
		assert.Equal(t, info.Variant, htd.Info.Variant)
		assert.Equal(t, info.TaskName, htd.Info.TaskName)
		assert.Equal(t, info.TestName, htd.Info.TestName)
		assert.Equal(t, info.RequestType, htd.Info.RequestType)
		assert.Equal(t, expectedDate, htd.Info.Date)
		assert.Zero(t, htd.NumPass)
		assert.Zero(t, htd.NumFail)
		assert.Zero(t, htd.AverageDuration)
		assert.Zero(t, htd.LastUpdate)
		assert.True(t, htd.populated)
	})
}

func TestHistoricalTestDataFind(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(historicalTestDataCollection).Drop(ctx))
	}()

	hd1 := getHistoricalTestData(t)
	hd2 := getHistoricalTestData(t)
	_, err := db.Collection(historicalTestDataCollection).InsertOne(ctx, hd1)
	require.NoError(t, err)

	t.Run("NoEnv", func(t *testing.T) {
		hd := &HistoricalTestData{Info: hd1.Info}
		assert.Error(t, hd.Find(ctx))
		assert.False(t, hd.populated)
	})
	t.Run("DNE", func(t *testing.T) {
		hd := &HistoricalTestData{Info: hd2.Info}
		hd.Setup(env)
		assert.False(t, hd.populated)
	})
	t.Run("Existing", func(t *testing.T) {
		hd := &HistoricalTestData{Info: hd1.Info}
		hd.Setup(env)
		require.NoError(t, hd.Find(ctx))
		assert.Equal(t, hd1.Info, hd.Info)
		assert.Equal(t, hd1.NumPass, hd.NumPass)
		assert.Equal(t, hd1.NumFail, hd.NumFail)
		assert.Equal(t, hd1.AverageDuration, hd.AverageDuration)
		assert.Equal(t, hd1.LastUpdate.UTC().Truncate(time.Millisecond), hd.LastUpdate)
		assert.True(t, hd.populated)
	})
}

func TestHistoricalTestDataUpdate(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(historicalTestDataCollection).Drop(ctx))
	}()
	info := getHistoricalTestData(t).Info

	t.Run("NoEnv", func(t *testing.T) {
		hd, err := CreateHistoricalTestData(info)
		require.NoError(t, err)

		assert.Error(t, hd.Update(ctx, TestResult{}))
	})
	t.Run("Unpopulated", func(t *testing.T) {
		hd, err := CreateHistoricalTestData(info)
		require.NoError(t, err)
		hd.Setup(env)
		hd.populated = false

		assert.Error(t, hd.Update(ctx, TestResult{}))
	})
	t.Run("UpsertAndUpdate", func(t *testing.T) {
		hd, err := CreateHistoricalTestData(info)
		require.NoError(t, err)
		hd.Setup(env)
		hd.populated = true
		now := time.Now()

		// upsert
		tr := TestResult{
			Status:        "pass",
			TestStartTime: now,
			TestEndTime:   now.Add(2 * time.Minute),
		}
		require.NoError(t, hd.Update(ctx, tr))

		actual := &HistoricalTestData{}
		require.NoError(t, db.Collection(historicalTestDataCollection).FindOne(ctx, bson.M{"_id": hd.ID}).Decode(actual))
		assert.Equal(t, 1, actual.NumPass)
		assert.Equal(t, 0, actual.NumFail)
		assert.Equal(t, 2*time.Minute, actual.AverageDuration)
		assert.True(t, time.Since(actual.LastUpdate) <= time.Second)

		// pass
		tr = TestResult{
			Status:        "pass",
			TestStartTime: now,
			TestEndTime:   now.Add(6 * time.Minute),
		}
		require.NoError(t, hd.Update(ctx, tr))

		actual = &HistoricalTestData{}
		require.NoError(t, db.Collection(historicalTestDataCollection).FindOne(ctx, bson.M{"_id": hd.ID}).Decode(actual))
		assert.Equal(t, 2, actual.NumPass)
		assert.Equal(t, 0, actual.NumFail)
		assert.Equal(t, 4*time.Minute, actual.AverageDuration)
		assert.True(t, time.Since(actual.LastUpdate) <= time.Second)

		// fail
		tr = TestResult{
			Status:        "fail",
			TestStartTime: now,
			TestEndTime:   now.Add(6 * time.Minute),
		}
		require.NoError(t, hd.Update(ctx, tr))

		actual = &HistoricalTestData{}
		require.NoError(t, db.Collection(historicalTestDataCollection).FindOne(ctx, bson.M{"_id": hd.ID}).Decode(actual))
		assert.Equal(t, 2, actual.NumPass)
		assert.Equal(t, 1, actual.NumFail)
		assert.Equal(t, 4*time.Minute, actual.AverageDuration)
		assert.True(t, time.Since(actual.LastUpdate) <= time.Second)

		// silent fail
		tr = TestResult{
			Status:        "silentfail",
			TestStartTime: now,
			TestEndTime:   now.Add(6 * time.Minute),
		}
		require.NoError(t, hd.Update(ctx, tr))

		actual = &HistoricalTestData{}
		require.NoError(t, db.Collection(historicalTestDataCollection).FindOne(ctx, bson.M{"_id": hd.ID}).Decode(actual))
		assert.Equal(t, 2, actual.NumPass)
		assert.Equal(t, 2, actual.NumFail)
		assert.Equal(t, 4*time.Minute, actual.AverageDuration)
		assert.True(t, time.Since(actual.LastUpdate) <= time.Second)
	})
}

func TestHistoricalTestDataRemove(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(historicalTestDataCollection).Drop(ctx))
	}()

	hd1 := getHistoricalTestData(t)
	hd2 := getHistoricalTestData(t)
	_, err := db.Collection(historicalTestDataCollection).InsertOne(ctx, hd1)
	require.NoError(t, err)

	t.Run("NoEnv", func(t *testing.T) {
		hd := &HistoricalTestData{Info: hd1.Info}

		assert.Error(t, hd.Remove(ctx))
		saved := &HistoricalTestData{}
		assert.NoError(t, db.Collection(historicalTestDataCollection).FindOne(ctx, bson.M{"_id": hd1.ID}).Decode(saved))
	})
	t.Run("DNE", func(t *testing.T) {
		hd := &HistoricalTestData{Info: hd2.Info}
		hd.Setup(env)

		assert.NoError(t, hd.Remove(ctx))
	})
	t.Run("RemoveData", func(t *testing.T) {
		hd := &HistoricalTestData{Info: hd1.Info}
		hd.Setup(env)

		require.NoError(t, hd.Remove(ctx))
		saved := &HistoricalTestData{}
		assert.Error(t, db.Collection(historicalTestDataCollection).FindOne(ctx, bson.M{"_id": hd1.ID}).Decode(saved))
	})
}

func getHistoricalTestData(t *testing.T) *HistoricalTestData {
	info := HistoricalTestDataInfo{
		Project:     utility.RandomString(),
		Variant:     utility.RandomString(),
		TaskName:    utility.RandomString(),
		TestName:    utility.RandomString(),
		RequestType: utility.RandomString(),
		Date:        time.Now().UTC().Round(time.Millisecond),
	}

	data, err := CreateHistoricalTestData(info)
	require.NoError(t, err)
	data.NumPass = rand.Intn(1000)
	var total float64
	data.NumFail = rand.Intn(1000)
	for i := 0; i < data.NumPass; i++ {
		total += rand.Float64() * 10000
	}
	data.AverageDuration = total / float64(data.NumPass)

	return data
}
