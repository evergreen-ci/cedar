package data

import (
	"context"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/utility"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistoricalTestDataConnectorDB(t *testing.T) {
	dbc, cleanup := newDBConnector(t)
	defer cleanup()

	t.Run("GetHistoricalTestData", func(t *testing.T) {
		expected := []model.APIAggregatedHistoricalTestData{
			{
				TestName:        utility.ToStringPtr("test1"),
				TaskName:        utility.ToStringPtr("task1"),
				Variant:         utility.ToStringPtr("v1"),
				Date:            model.NewTime(utility.GetUTCDay(time.Now())),
				NumPass:         1,
				AverageDuration: time.Second.Seconds(),
			},
			{
				TestName:        utility.ToStringPtr("test2"),
				TaskName:        utility.ToStringPtr("task2"),
				Variant:         utility.ToStringPtr("v1"),
				Date:            model.NewTime(utility.GetUTCDay(time.Now())),
				NumPass:         1,
				AverageDuration: time.Second.Seconds(),
			},
		}
		data, err := dbc.GetHistoricalTestData(context.TODO(), dbModel.HistoricalTestDataFilter{
			AfterDate:    utility.GetUTCDay(time.Now().Add(-24 * time.Hour)),
			BeforeDate:   utility.GetUTCDay(time.Now().Add(24 * time.Hour)),
			GroupNumDays: 1,
			Project:      "p1",
			Requesters:   []string{"r1", "r2"},
			Tests:        []string{"test1", "test2"},
			Tasks:        []string{"task1", "task2"},
			Variants:     []string{"v1", "v2"},
			GroupBy:      dbModel.HTDGroupByVariant,
			Sort:         dbModel.HTDSortEarliestFirst,
			Limit:        1000,
		})
		require.NoError(t, err)
		assert.Equal(t, expected, data)
	})
	t.Run("InvalidFilter", func(t *testing.T) {
		data, err := dbc.GetHistoricalTestData(context.TODO(), dbModel.HistoricalTestDataFilter{
			AfterDate:  utility.GetUTCDay(time.Now().Add(-24 * time.Hour)),
			BeforeDate: utility.GetUTCDay(time.Now().Add(24 * time.Hour)),
			Project:    "p1",
			Requesters: []string{"r1", "r2"},
			Tests:      []string{"test1", "test2"},
			Tasks:      []string{"task1", "task2"},
			Variants:   []string{"v1", "v2"},
			GroupBy:    dbModel.HTDGroupByVariant,
			Sort:       dbModel.HTDSortEarliestFirst,
			Limit:      1000,
		})
		assert.Error(t, err)
		assert.Nil(t, data)
	})
}

func TestHistoricalTestDataConnectorMock(t *testing.T) {
	mc := newMockConnector()
	expected := make([]model.APIAggregatedHistoricalTestData, len(mc.CachedHistoricalTestData))
	for i, data := range mc.CachedHistoricalTestData {
		require.NoError(t, expected[i].Import(data))
	}

	t.Run("GetHistoricalTestDataNoLimit", func(t *testing.T) {
		data, err := mc.GetHistoricalTestData(context.TODO(), dbModel.HistoricalTestDataFilter{})
		require.NoError(t, err)
		assert.Equal(t, expected, data)
	})
	t.Run("GetHistoricalTestDataLimit", func(t *testing.T) {
		data, err := mc.GetHistoricalTestData(context.TODO(), dbModel.HistoricalTestDataFilter{Limit: 1})
		require.NoError(t, err)
		assert.Equal(t, expected[:1], data)
	})
}

func newDBConnector(t *testing.T) (Connector, func()) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	require.NotNil(t, db)
	require.NoError(t, db.Drop(context.TODO()))

	data := []dbModel.HistoricalTestDataInfo{
		{
			Project:     "p1",
			Variant:     "v1",
			TaskName:    "task1",
			TestName:    "test1",
			RequestType: "r1",
			Date:        time.Now(),
		},
		{
			Project:     "p1",
			Variant:     "v1",
			TaskName:    "task2",
			TestName:    "test2",
			RequestType: "r1",
			Date:        time.Now(),
		},
	}
	for _, info := range data {
		htd, err := dbModel.CreateHistoricalTestData(info)
		require.NoError(t, err)
		now := time.Now()
		htd.Setup(env)
		require.NoError(t, htd.Update(context.TODO(), dbModel.TestResult{
			Status:        "pass",
			TestStartTime: now.Add(-time.Second),
			TestEndTime:   now,
		}))
	}

	return CreateNewDBConnector(env), func() { assert.NoError(t, db.Drop(context.TODO())) }
}

func newMockConnector() *MockConnector {
	return &MockConnector{
		CachedHistoricalTestData: []dbModel.AggregatedHistoricalTestData{
			{
				TestName:        "test1",
				TaskName:        "task1",
				Variant:         "v1",
				Date:            time.Now(),
				NumPass:         1,
				NumFail:         1,
				AverageDuration: time.Second,
			},
			{
				TestName:        "test2",
				TaskName:        "task1",
				Variant:         "v1",
				Date:            time.Now(),
				NumPass:         1,
				NumFail:         1,
				AverageDuration: time.Second,
			},
		},
		env: cedar.GetEnvironment(),
	}
}
