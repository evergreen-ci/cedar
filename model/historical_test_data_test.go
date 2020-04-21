package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateHistoricalTestData(t *testing.T) {
	info := HistoricalTestDataInfo{
		Project:   "project",
		Variant:   "variant",
		TaskName:  "task_name",
		TestName:  "test_name",
		Requester: "requester",
		Date:      time.Now(),
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
	t.Run("MissingRequester", func(t *testing.T) {
		requester := info.Requester
		info.Requester = ""
		htd, err := CreateHistoricalTestData(info)
		assert.Nil(t, htd)
		assert.Error(t, err)
		info.Requester = requester
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
		htd, err := CreateHistoricalTestData(info)
		assert.NoError(t, err)
		require.NotNil(t, htd)
		assert.Equal(t, info, htd.Info)
		assert.Zero(t, htd.NumPass)
		assert.Zero(t, htd.NumFail)
		assert.Zero(t, htd.AverageDuration)
		assert.True(t, time.Since(htd.LastUpdate) <= time.Second)
		assert.True(t, htd.populated)
	})
}
