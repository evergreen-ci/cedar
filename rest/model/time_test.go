package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeMarshal(t *testing.T) {
	utcLoc, err := time.LoadLocation("")
	require.NoError(t, err)
	TestTimeAsString := "\"2017-04-06T19:53:46.404Z\""
	TestTimeAsTime := time.Date(2017, time.April, 6, 19, 53, 46,
		404000000, utcLoc)
	t.Run("TestSameTimeZone", func(t *testing.T) {
		ti := NewTime(TestTimeAsTime)
		res, err := json.Marshal(ti)
		require.NoError(t, err)
		assert.Equal(t, string(res), TestTimeAsString)
	})
	t.Run("TestDifferentTimeZone", func(t *testing.T) {
		offsetZone := time.FixedZone("testZone", 10)

		ti := NewTime(TestTimeAsTime.In(offsetZone))
		res, err := json.Marshal(ti)
		require.NoError(t, err)
		assert.Equal(t, string(res), TestTimeAsString)
	})

}

func TestTimeUnmarshal(t *testing.T) {
	utcLoc, err := time.LoadLocation("")
	require.NoError(t, err)
	TestTimeAsString := "\"2017-04-06T19:53:46.404Z\""
	TestTimeAsTime := time.Date(2017, time.April, 6, 19, 53, 46,
		404000000, utcLoc)
	t.Run("TestNonNull", func(t *testing.T) {
		data := []byte(TestTimeAsString)
		res := APITime{}
		err := json.Unmarshal(data, &res)
		require.NoError(t, err)
		ti := NewTime(TestTimeAsTime)
		assert.Equal(t, res, ti)
	})
	t.Run("TestNull", func(t *testing.T) {
		data := []byte("null")
		res := APITime{}
		err := json.Unmarshal(data, &res)
		require.NoError(t, err)
		zt := time.Time(res)
		assert.True(t, zt.IsZero())
	})

}
