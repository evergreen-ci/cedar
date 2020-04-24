package model

import (
	"bytes"
	"context"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/pail"
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
		htd, err := CreateHistoricalTestData(info, PailLocal)
		assert.Nil(t, htd)
		assert.Error(t, err)
		info.Project = project
	})
	t.Run("MissingVariant", func(t *testing.T) {
		variant := info.Variant
		info.Variant = ""
		htd, err := CreateHistoricalTestData(info, PailLocal)
		assert.Nil(t, htd)
		assert.Error(t, err)
		info.Variant = variant
	})
	t.Run("MissingTaskName", func(t *testing.T) {
		taskName := info.TaskName
		info.TaskName = ""
		htd, err := CreateHistoricalTestData(info, PailLocal)
		assert.Nil(t, htd)
		assert.Error(t, err)
		info.TaskName = taskName
	})
	t.Run("MissingTestName", func(t *testing.T) {
		testName := info.TestName
		info.TestName = ""
		htd, err := CreateHistoricalTestData(info, PailLocal)
		assert.Nil(t, htd)
		assert.Error(t, err)
		info.TestName = testName
	})
	t.Run("MissingRequestType", func(t *testing.T) {
		RequestType := info.RequestType
		info.RequestType = ""
		htd, err := CreateHistoricalTestData(info, PailLocal)
		assert.Nil(t, htd)
		assert.Error(t, err)
		info.RequestType = RequestType
	})
	t.Run("MissingDate", func(t *testing.T) {
		date := info.Date
		info.Date = time.Time{}
		htd, err := CreateHistoricalTestData(info, PailLocal)
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
		htd, err := CreateHistoricalTestData(info, PailLocal)
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
		assert.True(t, time.Since(htd.LastUpdate) <= time.Second)
		assert.Equal(t, PailLocal, htd.ArtifactType)
		assert.True(t, htd.populated)
	})
}

func TestHistoricalTestDataFind(t *testing.T) {
	env := cedar.GetEnvironment()
	db := env.GetDB()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tmpDir, err := ioutil.TempDir(".", "find-test")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
		assert.NoError(t, db.Collection(configurationCollection).Drop(ctx))
	}()

	testBucket, err := pail.NewLocalBucket(pail.LocalOptions{Path: tmpDir})
	require.NoError(t, err)
	hd1 := getHistoricalTestData(t)
	hd2 := getHistoricalTestData(t)
	data, err := bson.Marshal(hd1)
	require.NoError(t, err)
	require.NoError(t, testBucket.Put(ctx, hd1.Info.getPath(), bytes.NewReader(data)))

	t.Run("NoConfig", func(t *testing.T) {
		hd := &HistoricalTestData{Info: hd1.Info, ArtifactType: hd1.ArtifactType}
		hd.Setup(env)
		assert.Error(t, hd.Find(ctx))
		assert.False(t, hd.populated)
	})
	conf := &CedarConfig{populated: true}
	conf.Setup(env)
	require.NoError(t, conf.Save())
	t.Run("ConfigWithoutBucket", func(t *testing.T) {
		hd := &HistoricalTestData{Info: hd1.Info, ArtifactType: hd1.ArtifactType}
		hd.Setup(env)
		assert.Error(t, hd.Find(ctx))
		assert.False(t, hd.populated)
	})
	conf.Setup(env)
	require.NoError(t, conf.Find())
	conf.Bucket.HistoricalTestStatsBucket = tmpDir
	require.NoError(t, conf.Save())
	t.Run("NoEnv", func(t *testing.T) {
		hd := &HistoricalTestData{Info: hd1.Info, ArtifactType: hd1.ArtifactType}
		assert.Error(t, hd.Find(ctx))
		assert.False(t, hd.populated)
	})
	t.Run("DNE", func(t *testing.T) {
		hd := &HistoricalTestData{Info: hd2.Info, ArtifactType: hd2.ArtifactType}
		hd.Setup(env)
		assert.False(t, hd.populated)
	})
	t.Run("Existing", func(t *testing.T) {
		hd := &HistoricalTestData{Info: hd1.Info, ArtifactType: hd1.ArtifactType}
		hd.Setup(env)
		require.NoError(t, hd.Find(ctx))
		assert.Equal(t, hd1.Info, hd.Info)
		assert.Equal(t, hd1.NumPass, hd.NumPass)
		assert.Equal(t, hd1.NumFail, hd.NumFail)
		assert.Equal(t, hd1.Durations, hd.Durations)
		assert.Equal(t, hd1.AverageDuration, hd.AverageDuration)
		assert.True(t, hd.populated)
	})
}

func getHistoricalTestData(t *testing.T) *HistoricalTestData {
	info := HistoricalTestDataInfo{
		Project:     utility.RandomString(),
		Variant:     utility.RandomString(),
		TaskName:    utility.RandomString(),
		TestName:    utility.RandomString(),
		RequestType: utility.RandomString(),
		Date:        time.Now(),
	}

	data, err := CreateHistoricalTestData(info, PailLocal)
	require.NoError(t, err)
	data.NumPass = rand.Intn(1000)
	data.NumFail = rand.Intn(1000)
	total := 0.0
	durations := make([]float64, data.NumPass+data.NumFail)
	for i := 0; i < data.NumPass+data.NumFail; i++ {
		durations[i] = rand.Float64() * 10000
		total += durations[i]
	}
	data.Durations = durations
	data.AverageDuration = total / float64(data.NumPass+data.NumFail)

	return data
}
