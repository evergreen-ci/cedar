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

func TestHTDGroupByValidate(t *testing.T) {
	for _, test := range []struct {
		name    string
		groupBy HTDGroupBy
		hasErr  bool
	}{
		{
			name:    "Invalid",
			groupBy: "invalid",
			hasErr:  true,
		},
		{
			name:    "Test",
			groupBy: HTDGroupByTest,
		},
		{
			name:    "Task",
			groupBy: HTDGroupByTask,
		},
		{
			name:    "Variant",
			groupBy: HTDGroupByVariant,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := test.groupBy.validate()
			if test.hasErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHTDSortValidate(t *testing.T) {
	for _, test := range []struct {
		name   string
		sort   HTDSort
		hasErr bool
	}{
		{
			name:   "Invalid",
			sort:   "invalid",
			hasErr: true,
		},
		{
			name: "Earliest",
			sort: HTDSortEarliestFirst,
		},
		{
			name: "Latest",
			sort: HTDSortLatestFirst,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := test.sort.validate()
			if test.hasErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHTDStartAtValidate(t *testing.T) {
	for _, test := range []struct {
		name    string
		startAt *HTDStartAt
		groupBy HTDGroupBy
		hasErr  bool
	}{
		{
			name: "InvalidDate",
			startAt: &HTDStartAt{
				Date:    time.Date(2020, time.December, 18, 5, 0, 0, 0, time.UTC),
				Variant: "variant",
				Task:    "task",
				Test:    "test",
			},
			hasErr: true,
		},
		{
			name: "MissingTest",
			startAt: &HTDStartAt{
				Date:    utility.GetUTCDay(time.Now()),
				Variant: "variant",
				Task:    "task",
			},
			hasErr: true,
		},
		{
			name: "MissingVariantWhenGroupByVariant",
			startAt: &HTDStartAt{
				Date: utility.GetUTCDay(time.Now()),
				Task: "task",
				Test: "test",
			},
			groupBy: HTDGroupByVariant,
			hasErr:  true,
		},
		{
			name: "MissingTaskWhenGroupByVariant",
			startAt: &HTDStartAt{
				Date:    utility.GetUTCDay(time.Now()),
				Variant: "variant",
				Test:    "test",
			},
			groupBy: HTDGroupByVariant,
			hasErr:  true,
		},
		{
			name: "MissingTaskWhenGroupByTask",
			startAt: &HTDStartAt{
				Date:    utility.GetUTCDay(time.Now()),
				Variant: "variant",
				Test:    "test",
			},
			groupBy: HTDGroupByTask,
			hasErr:  true,
		},

		{
			name: "GroupByTaskNoVariant",
			startAt: &HTDStartAt{
				Date: utility.GetUTCDay(time.Now()),
				Task: "task",
				Test: "test",
			},
			groupBy: HTDGroupByTask,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := test.startAt.validate(test.groupBy)
			if test.hasErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetHistoricalTestData(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		assert.NoError(t, db.Collection(historicalTestDataCollection).Drop(ctx))
	}()

	for _, test := range []struct {
		name     string
		filter   func() HistoricalTestDataFilter
		data     []interface{}
		expected []AggregatedHistoricalTestData
	}{
		{
			name:   "EmptyCollection",
			filter: getBaseHTDFilter,
		},
		{
			name:   "OneDocument",
			filter: getBaseHTDFilter,
			data: []interface{}{
				&HistoricalTestData{
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         10,
					NumFail:         2,
					AverageDuration: time.Second,
				},
			},
			expected: []AggregatedHistoricalTestData{
				{
					TestName:        "test1",
					TaskName:        "task1",
					Variant:         "v1",
					Date:            day1,
					NumPass:         10,
					NumFail:         2,
					AverageDuration: time.Second,
				},
			},
		},
		{
			name:   "TwoDocuments",
			filter: getBaseHTDFilter,
			data: []interface{}{
				&HistoricalTestData{
					ID: "one",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         10,
					NumFail:         2,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "two",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day2,
					},
					NumPass:         20,
					NumFail:         7,
					AverageDuration: 2 * time.Second,
				},
			},
			expected: []AggregatedHistoricalTestData{
				{
					TestName:        "test1",
					TaskName:        "task1",
					Variant:         "v1",
					Date:            day1,
					NumPass:         10,
					NumFail:         2,
					AverageDuration: time.Second,
				},
				{
					TestName:        "test1",
					TaskName:        "task1",
					Variant:         "v1",
					Date:            day2,
					NumPass:         20,
					NumFail:         7,
					AverageDuration: 2 * time.Second,
				},
			},
		},
		{
			name:   "FilterScope",
			filter: getBaseHTDFilter,
			data: []interface{}{
				&HistoricalTestData{
					ID: "one",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         10,
					NumFail:         2,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "two",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day2,
					},
					NumPass:         20,
					NumFail:         7,
					AverageDuration: 2 * time.Second,
				},
				&HistoricalTestData{
					ID: "three",
					Info: HistoricalTestDataInfo{
						Project:     "p3",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "four",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r3",
						Date:        day1,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "five",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test3",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "six",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task3",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "seven",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v3",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "eight",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v3",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day8,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
			},
			expected: []AggregatedHistoricalTestData{
				{
					TestName:        "test1",
					TaskName:        "task1",
					Variant:         "v1",
					Date:            day1,
					NumPass:         10,
					NumFail:         2,
					AverageDuration: time.Second,
				},
				{
					TestName:        "test1",
					TaskName:        "task1",
					Variant:         "v1",
					Date:            day2,
					NumPass:         20,
					NumFail:         7,
					AverageDuration: 2 * time.Second,
				},
			},
		},
		{
			name: "SortEarliestFirst",
			filter: func() HistoricalTestDataFilter {
				filter := getBaseHTDFilter()
				filter.Sort = HTDSortEarliestFirst
				return filter
			},
			data: []interface{}{
				&HistoricalTestData{
					ID: "one",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         10,
					NumFail:         2,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "two",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day2,
					},
					NumPass:         20,
					NumFail:         7,
					AverageDuration: 2 * time.Second,
				},
			},
			expected: []AggregatedHistoricalTestData{
				{
					TestName:        "test1",
					TaskName:        "task1",
					Variant:         "v1",
					Date:            day1,
					NumPass:         10,
					NumFail:         2,
					AverageDuration: time.Second,
				},
				{
					TestName:        "test1",
					TaskName:        "task1",
					Variant:         "v1",
					Date:            day2,
					NumPass:         20,
					NumFail:         7,
					AverageDuration: 2 * time.Second,
				},
			},
		},
		{
			name: "SortLatestFirst",
			filter: func() HistoricalTestDataFilter {
				filter := getBaseHTDFilter()
				filter.Sort = HTDSortLatestFirst
				return filter
			},
			data: []interface{}{
				&HistoricalTestData{
					ID: "one",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         10,
					NumFail:         2,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "two",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day2,
					},
					NumPass:         20,
					NumFail:         7,
					AverageDuration: 2 * time.Second,
				},
			},
			expected: []AggregatedHistoricalTestData{
				{
					TestName:        "test1",
					TaskName:        "task1",
					Variant:         "v1",
					Date:            day2,
					NumPass:         20,
					NumFail:         7,
					AverageDuration: 2 * time.Second,
				},
				{
					TestName:        "test1",
					TaskName:        "task1",
					Variant:         "v1",
					Date:            day1,
					NumPass:         10,
					NumFail:         2,
					AverageDuration: time.Second,
				},
			},
		},
		{
			name: "GroupNumDays",
			filter: func() HistoricalTestDataFilter {
				filter := getBaseHTDFilter()
				filter.GroupNumDays = 2
				return filter
			},
			data: []interface{}{
				&HistoricalTestData{
					ID: "one",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         10,
					NumFail:         2,
					AverageDuration: 6 * time.Second,
				},
				&HistoricalTestData{
					ID: "two",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day2,
					},
					NumPass:         10,
					NumFail:         7,
					AverageDuration: 5 * time.Second,
				},
			},
			expected: []AggregatedHistoricalTestData{
				{
					TestName:        "test1",
					TaskName:        "task1",
					Variant:         "v1",
					Date:            day1,
					NumPass:         20,
					NumFail:         9,
					AverageDuration: 5500 * time.Millisecond,
				},
			},
		},
		{
			name: "GroupByVariant",
			filter: func() HistoricalTestDataFilter {
				filter := getBaseHTDFilter()
				filter.GroupNumDays = 2
				filter.GroupBy = HTDGroupByVariant
				return filter
			},
			data: []interface{}{
				&HistoricalTestData{
					ID: "one",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         10,
					NumFail:         2,
					AverageDuration: 6 * time.Second,
				},
				&HistoricalTestData{
					ID: "two",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day2,
					},
					NumPass:         10,
					NumFail:         7,
					AverageDuration: 5 * time.Second,
				},
				&HistoricalTestData{
					ID: "three",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v2",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day2,
					},
					NumPass:         2,
					NumFail:         2,
					AverageDuration: time.Second,
				},
			},
			expected: []AggregatedHistoricalTestData{
				{
					TestName:        "test1",
					TaskName:        "task1",
					Variant:         "v1",
					Date:            day1,
					NumPass:         20,
					NumFail:         9,
					AverageDuration: 5500 * time.Millisecond,
				},
				{
					TestName:        "test1",
					TaskName:        "task1",
					Variant:         "v2",
					Date:            day1,
					NumPass:         2,
					NumFail:         2,
					AverageDuration: time.Second,
				},
			},
		},
		{
			name: "GroupByTask",
			filter: func() HistoricalTestDataFilter {
				filter := getBaseHTDFilter()
				filter.GroupNumDays = 2
				filter.GroupBy = HTDGroupByTask
				return filter
			},
			data: []interface{}{
				&HistoricalTestData{
					ID: "one",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         10,
					NumFail:         2,
					AverageDuration: 6 * time.Second,
				},
				&HistoricalTestData{
					ID: "two",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day2,
					},
					NumPass:         10,
					NumFail:         7,
					AverageDuration: 5 * time.Second,
				},
				&HistoricalTestData{
					ID: "three",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v2",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day2,
					},
					NumPass:         4,
					NumFail:         2,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "four",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v2",
						TaskName:    "task2",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day2,
					},
					NumPass:         2,
					NumFail:         2,
					AverageDuration: time.Second,
				},
			},
			expected: []AggregatedHistoricalTestData{
				{
					TestName:        "test1",
					TaskName:        "task1",
					Date:            day1,
					NumPass:         24,
					NumFail:         11,
					AverageDuration: 4750 * time.Millisecond,
				},
				{
					TestName:        "test1",
					TaskName:        "task2",
					Date:            day1,
					NumPass:         2,
					NumFail:         2,
					AverageDuration: time.Second,
				},
			},
		},
		{
			name: "GroupByTest",
			filter: func() HistoricalTestDataFilter {
				filter := getBaseHTDFilter()
				filter.GroupNumDays = 2
				filter.GroupBy = HTDGroupByTest
				return filter
			},
			data: []interface{}{
				&HistoricalTestData{
					ID: "one",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         10,
					NumFail:         2,
					AverageDuration: 6 * time.Second,
				},
				&HistoricalTestData{
					ID: "two",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day2,
					},
					NumPass:         10,
					NumFail:         7,
					AverageDuration: 5 * time.Second,
				},
				&HistoricalTestData{
					ID: "three",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v2",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day2,
					},
					NumPass:         4,
					NumFail:         2,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "four",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v2",
						TaskName:    "task2",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day2,
					},
					NumPass:         6,
					NumFail:         2,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "five",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v2",
						TaskName:    "task2",
						TestName:    "test2",
						RequestType: "r1",
						Date:        day2,
					},
					NumPass:         2,
					NumFail:         2,
					AverageDuration: time.Second,
				},
			},
			expected: []AggregatedHistoricalTestData{
				{
					TestName:        "test1",
					Date:            day1,
					NumPass:         30,
					NumFail:         13,
					AverageDuration: 4 * time.Second,
				},
				{
					TestName:        "test2",
					Date:            day1,
					NumPass:         2,
					NumFail:         2,
					AverageDuration: time.Second,
				},
			},
		},
		{
			name: "Limit",
			filter: func() HistoricalTestDataFilter {
				filter := getBaseHTDFilter()
				filter.GroupBy = HTDGroupByVariant
				filter.GroupNumDays = 2
				filter.Limit = 3
				return filter
			},
			data: []interface{}{
				&HistoricalTestData{
					ID: "one",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         10,
					NumFail:         2,
					AverageDuration: 2 * time.Second,
				},
				&HistoricalTestData{
					ID: "two",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test2",
						RequestType: "r1",
						Date:        day2,
					},
					NumPass:         20,
					NumFail:         7,
					AverageDuration: 2 * time.Second,
				},
				&HistoricalTestData{
					ID: "three",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v2",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         20,
					NumFail:         7,
					AverageDuration: 2 * time.Second,
				},
				&HistoricalTestData{
					ID: "four",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v2",
						TaskName:    "task1",
						TestName:    "test2",
						RequestType: "r1",
						Date:        day2,
					},
					NumPass:         2,
					NumFail:         2,
					AverageDuration: time.Second,
				},
			},
			expected: []AggregatedHistoricalTestData{
				{
					TestName:        "test1",
					TaskName:        "task1",
					Variant:         "v1",
					Date:            day1,
					NumPass:         10,
					NumFail:         2,
					AverageDuration: 2 * time.Second,
				},
				{
					TestName:        "test2",
					TaskName:        "task1",
					Variant:         "v1",
					Date:            day1,
					NumPass:         20,
					NumFail:         7,
					AverageDuration: 2 * time.Second,
				},
				{
					TestName:        "test1",
					TaskName:        "task1",
					Variant:         "v2",
					Date:            day1,
					NumPass:         20,
					NumFail:         7,
					AverageDuration: 2 * time.Second,
				},
			},
		},
		{
			name: "Pagination",
			filter: func() HistoricalTestDataFilter {
				filter := getBaseHTDFilter()
				filter.Tests = nil
				filter.GroupBy = HTDGroupByVariant
				filter.StartAt = &HTDStartAt{
					Date:    day1,
					Test:    "test2",
					Task:    "task2",
					Variant: "v1",
				}
				return filter
			},
			data: []interface{}{
				&HistoricalTestData{
					ID: "one",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v2",
						TaskName:    "task2",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "two",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task2",
						TestName:    "test2",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "three",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v2",
						TaskName:    "task2",
						TestName:    "test3",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "four",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test4",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "five",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v2",
						TaskName:    "task1",
						TestName:    "test5",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "six",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test6",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "seven",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day2,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
			},
			expected: []AggregatedHistoricalTestData{
				{
					TestName:        "test2",
					TaskName:        "task2",
					Variant:         "v1",
					Date:            day1,
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				{
					TestName:        "test5",
					TaskName:        "task1",
					Variant:         "v2",
					Date:            day1,
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				{
					TestName:        "test1",
					TaskName:        "task2",
					Variant:         "v2",
					Date:            day1,
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				{
					TestName:        "test3",
					TaskName:        "task2",
					Variant:         "v2",
					Date:            day1,
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				{
					TestName:        "test1",
					TaskName:        "task1",
					Variant:         "v1",
					Date:            day2,
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
			},
		},
		{
			name: "PaginationGroupByTest",
			filter: func() HistoricalTestDataFilter {
				filter := getBaseHTDFilter()
				filter.Tests = nil
				filter.GroupBy = HTDGroupByTest
				filter.StartAt = &HTDStartAt{Date: day1, Test: "test3"}
				return filter
			},
			data: []interface{}{
				&HistoricalTestData{
					ID: "one",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v2",
						TaskName:    "task2",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "two",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task2",
						TestName:    "test2",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "three",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v2",
						TaskName:    "task1",
						TestName:    "test3",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "four",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test4",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "five",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v2",
						TaskName:    "task1",
						TestName:    "test5",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "six",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test6",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
			},
			expected: []AggregatedHistoricalTestData{
				{
					TestName:        "test3",
					Date:            day1,
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				{
					TestName:        "test4",
					Date:            day1,
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				{
					TestName:        "test5",
					Date:            day1,
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				{
					TestName:        "test6",
					Date:            day1,
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
			},
		},
		{
			name: "PaginationGroupNumDays",
			filter: func() HistoricalTestDataFilter {
				filter := getBaseHTDFilter()
				filter.Tests = nil
				filter.GroupNumDays = 7
				filter.BeforeDate = day8.Add(7 * 24 * time.Hour)
				filter.StartAt = &HTDStartAt{
					Date:    day1,
					Test:    "test3",
					Task:    "task1",
					Variant: "v1",
				}
				return filter
			},
			data: []interface{}{
				&HistoricalTestData{
					ID: "one",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "two",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test2",
						RequestType: "r1",
						Date:        day1,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "three",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day2,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "four",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test3",
						RequestType: "r1",
						Date:        day2,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "five",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test4",
						RequestType: "r1",
						Date:        day2,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "six",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test1",
						RequestType: "r1",
						Date:        day8,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				&HistoricalTestData{
					ID: "seven",
					Info: HistoricalTestDataInfo{
						Project:     "p1",
						Variant:     "v1",
						TaskName:    "task1",
						TestName:    "test2",
						RequestType: "r1",
						Date:        day8,
					},
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
			},
			expected: []AggregatedHistoricalTestData{
				{
					TestName:        "test3",
					TaskName:        "task1",
					Variant:         "v1",
					Date:            day1,
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				{
					TestName:        "test4",
					TaskName:        "task1",
					Variant:         "v1",
					Date:            day1,
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				{
					TestName:        "test1",
					TaskName:        "task1",
					Variant:         "v1",
					Date:            day8,
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
				{
					TestName:        "test2",
					TaskName:        "task1",
					Variant:         "v1",
					Date:            day8,
					NumPass:         1,
					NumFail:         1,
					AverageDuration: time.Second,
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.NoError(t, db.Collection(historicalTestDataCollection).Drop(ctx))
			if len(test.data) > 0 {
				_, err := db.Collection(historicalTestDataCollection).InsertMany(ctx, test.data)
				require.NoError(t, err)
			}

			docs, err := GetHistoricalTestData(ctx, env, test.filter())
			require.NoError(t, err)
			assert.Equal(t, test.expected, docs)
		})
	}
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
	data.NumPass = rand.Intn(1000) + 1
	var total time.Duration
	data.NumFail = rand.Intn(1000)
	for i := 0; i < data.NumPass; i++ {
		total += time.Duration(rand.Float64()) * 10000
	}
	data.AverageDuration = total / time.Duration(data.NumPass)

	return data
}

var (
	baseDay = time.Date(2018, 7, 15, 0, 0, 0, 0, time.UTC)
	day1    = baseDay
	day2    = baseDay.Add(24 * time.Hour)
	day8    = baseDay.Add(7 * 24 * time.Hour)
)

func getBaseHTDFilter() HistoricalTestDataFilter {
	return HistoricalTestDataFilter{
		AfterDate:    day1,
		BeforeDate:   day8,
		GroupNumDays: 1,
		Project:      "p1",
		Requesters:   []string{"r1", "r2"},
		Tests:        []string{"test1", "test2"},
		Tasks:        []string{"task1", "task2"},
		Variants:     []string{"v1", "v2"},
		GroupBy:      HTDGroupByVariant,
		Sort:         HTDSortEarliestFirst,
		Limit:        htdMaxQueryLimit,
	}
}
