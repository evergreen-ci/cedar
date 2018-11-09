package model

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestString(t *testing.T) {
	t.Run("TestMarshal", func(t *testing.T) {
		t.Run("TestEmptyString", func(t *testing.T) {
			str := ToAPIString("")
			data, err := json.Marshal(str)
			require.NoError(t, err)
			assert.Equal(t, string(data), "\"\"")
		})
		t.Run("NonemptyString", func(t *testing.T) {
			testStr := "testStr"
			str := ToAPIString(testStr)
			data, err := json.Marshal(str)
			require.NoError(t, err)
			assert.Equal(t, string(data), fmt.Sprintf("\"%s\"", testStr))
		})

	})
	t.Run("TestUnmarshal", func(t *testing.T) {
		var res APIString
		testStr := "testStr"
		str := fmt.Sprintf("\"%s\"", testStr)
		err := json.Unmarshal([]byte(str), &res)
		require.NoError(t, err)
		assert.Equal(t, FromAPIString(res), testStr)
	})
}
