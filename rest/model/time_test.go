package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeMarshal(t *testing.T) {
	t.Run("When checking time", func(t *testing.T) {
		utcLoc, err := time.LoadLocation("")
		require.NoError(t, err)
		TestTimeAsString := "\"2017-04-06T19:53:46.404Z\""
		TestTimeAsTime := time.Date(2017, time.April, 6, 19, 53, 46,
			404000000, utcLoc)
		t.Run("then time in same time zone should marshal correctly", func(t *testing.T) {
			ti := NewTime(TestTimeAsTime)
			res, err := json.Marshal(ti)
			require.NoError(t, err)
			assert.Equal(t, string(res), TestTimeAsString)
		})
		t.Run("then time in different time zone should marshal correctly", func(t *testing.T) {
			offsetZone := time.FixedZone("testZone", 10)

			ti := NewTime(TestTimeAsTime.In(offsetZone))
			res, err := json.Marshal(ti)
			require.NoError(t, err)
			assert.Equal(t, string(res), TestTimeAsString)
		})

	})

}

func TestTimeUnmarshal(t *testing.T) {
	t.Run("When checking time", func(t *testing.T) {
		utcLoc, err := time.LoadLocation("")
		require.NoError(t, err)
		TestTimeAsString := "\"2017-04-06T19:53:46.404Z\""
		TestTimeAsTime := time.Date(2017, time.April, 6, 19, 53, 46,
			404000000, utcLoc)
		t.Run("then time should unmarshal correctly when non null", func(t *testing.T) {
			data := []byte(TestTimeAsString)
			res := APITime{}
			err := json.Unmarshal(data, &res)
			require.NoError(t, err)
			ti := NewTime(TestTimeAsTime)
			assert.Equal(t, res, ti)
		})
		t.Run("then time should unmarshal correctly when null", func(t *testing.T) {
			data := []byte("null")
			res := APITime{}
			err := json.Unmarshal(data, &res)
			require.NoError(t, err)
			zt := time.Time(res)
			assert.True(t, zt.IsZero())
		})
	})

}
