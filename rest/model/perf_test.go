package model

import (
	"testing"

	dbmodel "github.com/evergreen-ci/sink/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHelperFunctions(t *testing.T) {
	for _, test := range []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name: "TestgetPerformanceResultID",
			input: dbmodel.PerformanceResultID{
				Project:   "project",
				Version:   "version",
				TaskName:  "taskname",
				TaskID:    "taskid",
				Execution: 0,
				TestName:  "testname",
				Trial:     0,
				Parent:    "parent",
				Tags:      []string{"tag0", "tag1", "tag2"},
				Arguments: map[string]int32{
					"argument0": 0,
					"argument1": 1,
					"argument2": 2,
				},
				Schema: 0,
			},
			expected: APIPerformanceResultID{
				Project:   ToAPIString("project"),
				Version:   ToAPIString("version"),
				TaskName:  ToAPIString("tasname"),
				TaskID:    ToAPIString("taskid"),
				Execution: 0,
				TestName:  ToAPIString("testname"),
				Trial:     0,
				Parent:    ToAPIString("parent"),
				Tags:      []string{"tag0", "tag1", "tag2"},
				Arguments: map[string]int32{
					"argument0": 0,
					"argument1": 1,
					"argument2": 2,
				},
				Schema: 0,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			switch i := test.input.(type) {
			case dbmodel.PerformanceResultID:
				output, err := getPerformanceResultID(i)
				require.NoError(t, err)
				assert.Equal(t, output, test.expected)
			}
		})
	}
}
