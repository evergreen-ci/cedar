package model

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringMarshal(t *testing.T) {
	t.Run("When checking string", func(t *testing.T) {
		t.Run("and marshalling", func(t *testing.T) {
			t.Run("then empty string should produce an empty string as output", func(t *testing.T) {
				str := ToAPIString("")
				data, err := json.Marshal(str)
				require.NoError(t, err)
				assert.Equal(t, string(data), "\"\"")
			})
			t.Run("then non empty string should produce regular output", func(t *testing.T) {
				testStr := "testStr"
				str := ToAPIString(testStr)
				data, err := json.Marshal(str)
				require.NoError(t, err)
				assert.Equal(t, string(data), fmt.Sprintf("\"%s\"", testStr))
			})

		})
		t.Run("and unmarshalling", func(t *testing.T) {
			t.Run("then non empty string should produce regular output", func(t *testing.T) {
				var res APIString
				testStr := "testStr"
				str := fmt.Sprintf("\"%s\"", testStr)
				err := json.Unmarshal([]byte(str), &res)
				require.NoError(t, err)
				assert.Equal(t, FromAPIString(res), testStr)
			})
		})
	})
}
