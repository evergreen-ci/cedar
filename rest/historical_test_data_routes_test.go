package rest

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	dbModel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/utility"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTDFilterHandlerParse(t *testing.T) {
	values := url.Values{
		"requesters":  []string{htdAPIRequesterMainline, htdAPIRequesterPatch},
		"after_date":  []string{"1998-07-12"},
		"before_date": []string{"2018-07-15"},
		"tests":       []string{"test1", "test2"},
		"tasks":       []string{"task1", "task2"},
		"variants":    []string{"v1,v2", "v3"},
	}
	handler := htdFilterHandler{}

	err := handler.parse(values)
	require.NoError(t, err)

	assert.Equal(t, []string{
		cedar.RepotrackerVersionRequester,
		cedar.PatchVersionRequester,
		cedar.GithubPRRequester,
		cedar.MergeTestRequester,
	}, handler.filter.Requesters)
	assert.Equal(t, time.Date(1998, 7, 12, 0, 0, 0, 0, time.UTC), handler.filter.AfterDate)
	assert.Equal(t, time.Date(2018, 7, 15, 0, 0, 0, 0, time.UTC), handler.filter.BeforeDate)
	assert.Equal(t, values["tests"], handler.filter.Tests)
	assert.Equal(t, values["tasks"], handler.filter.Tasks)
	assert.Equal(t, []string{"v1", "v2", "v3"}, handler.filter.Variants)
	assert.Nil(t, handler.filter.StartAt)
	assert.Equal(t, dbModel.HTDGroupByVariant, handler.filter.GroupBy) // default value
	assert.Equal(t, dbModel.HTDSortEarliestFirst, handler.filter.Sort) // default value
	assert.Equal(t, htdAPIMaxLimit+1, handler.filter.Limit)            // default value
}

func TestHTDDataHandlerRun(t *testing.T) {
	var err error
	sc := &data.MockConnector{
		CachedHistoricalTestData: []dbModel.AggregatedHistoricalTestData{
			{
				TestName: "test1",
				TaskName: "task1",
				Variant:  "v1",
				Date:     utility.GetUTCDay(time.Now()),
			},
			{
				TestName: "test2",
				TaskName: "task2",
				Variant:  "v1",
				Date:     utility.GetUTCDay(time.Now()),
			},
			{
				TestName: "test3",
				TaskName: "task3",
				Variant:  "v1",
				Date:     utility.GetUTCDay(time.Now()),
			},
		},
	}
	handler := makeGetHistoricalTestData(sc).(*historicalTestDataHandler)
	require.NoError(t, err)

	handler.filter = dbModel.HistoricalTestDataFilter{Limit: 4}
	resp := handler.Run(context.Background())
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.Status())
	assert.Nil(t, resp.Pages())

	// pagination
	handler.filter = dbModel.HistoricalTestDataFilter{Limit: 3}
	resp = handler.Run(context.Background())
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.Status())
	assert.NotNil(t, resp.Pages())
	lastDoc := model.APIAggregatedHistoricalTestData{}
	require.NoError(t, lastDoc.Import(sc.CachedHistoricalTestData[2]))
	assert.Equal(t, lastDoc.StartAtKey(), resp.Pages().Next.Key)

	// no data
	sc.CachedHistoricalTestData = []dbModel.AggregatedHistoricalTestData{}
	resp = handler.Run(context.Background())
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusNotFound, resp.Status())
	assert.Nil(t, resp.Data)
}

func TestHTDReadStartAt(t *testing.T) {

	// 5 values
	handler := htdFilterHandler{}
	startAt, err := handler.readStartAt("1998-07-12|variant1|task1|test1|distro1")
	require.NoError(t, err)
	assert.Equal(t, time.Date(1998, 7, 12, 0, 0, 0, 0, time.UTC), startAt.Date)
	assert.Equal(t, "variant1", startAt.Variant)
	assert.Equal(t, "task1", startAt.Task)
	assert.Equal(t, "test1", startAt.Test)

	// 4 values
	handler = htdFilterHandler{}
	startAt, err = handler.readStartAt("1998-07-12|variant1|task1|test1")
	require.NoError(t, err)
	assert.Equal(t, time.Date(1998, 7, 12, 0, 0, 0, 0, time.UTC), startAt.Date)
	assert.Equal(t, "variant1", startAt.Variant)
	assert.Equal(t, "task1", startAt.Task)
	assert.Equal(t, "test1", startAt.Test)

	// Invalid format
	_, err = handler.readStartAt("1998-07-12|variant1|task1")
	require.Error(t, err)
}
