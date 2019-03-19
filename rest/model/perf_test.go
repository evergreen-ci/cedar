package model

import (
	"fmt"
	"testing"
	"time"

	dbmodel "github.com/evergreen-ci/cedar/model"
	"github.com/mongodb/ftdc/events"
	"github.com/stretchr/testify/assert"
)

func TestImportHelperFunctions(t *testing.T) {
	for _, test := range []struct {
		name           string
		input          interface{}
		expectedOutput interface{}
	}{
		{
			name: "GetPerformanceResultInfo",
			input: dbmodel.PerformanceResultInfo{
				Project:   "project",
				Version:   "version",
				TaskName:  "taskname",
				TaskID:    "taskid",
				Execution: 1,
				TestName:  "testname",
				Trial:     1,
				Parent:    "parent",
				Tags:      []string{"tag0", "tag1", "tag2"},
				Arguments: map[string]int32{
					"argument0": 0,
					"argument1": 1,
					"argument2": 2,
				},
				Schema: 1,
			},
			expectedOutput: APIPerformanceResultInfo{
				Project:   ToAPIString("project"),
				Version:   ToAPIString("version"),
				TaskName:  ToAPIString("taskname"),
				TaskID:    ToAPIString("taskid"),
				Execution: 1,
				TestName:  ToAPIString("testname"),
				Trial:     1,
				Parent:    ToAPIString("parent"),
				Tags:      []string{"tag0", "tag1", "tag2"},
				Arguments: map[string]int32{
					"argument0": 0,
					"argument1": 1,
					"argument2": 2,
				},
				Schema: 1,
			},
		},
		{
			name: "GetArtifactInfo",
			input: dbmodel.ArtifactInfo{
				Type:        dbmodel.PailS3,
				Bucket:      "bucket",
				Path:        "path",
				Format:      dbmodel.FileFTDC,
				Compression: dbmodel.FileZip,
				Schema:      dbmodel.SchemaCollapsedEvents,
				Tags:        []string{"tag0", "tag1", "tag2"},
				CreatedAt:   time.Date(2018, time.December, 31, 23, 59, 59, 0, time.UTC),
			},
			expectedOutput: APIArtifactInfo{
				Type:        ToAPIString(string(dbmodel.PailS3)),
				Bucket:      ToAPIString("bucket"),
				Path:        ToAPIString("path"),
				Format:      ToAPIString(string(dbmodel.FileFTDC)),
				Compression: ToAPIString(string(dbmodel.FileZip)),
				Schema:      ToAPIString(string(dbmodel.SchemaCollapsedEvents)),
				Tags:        []string{"tag0", "tag1", "tag2"},
				CreatedAt:   NewTime(time.Date(2018, time.December, 31, 23, 59, 59, 0, time.UTC)),
			},
		},
		{
			name: "GetPerformanceEvent",
			input: &events.Performance{
				Timestamp: time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC),
				Counters: events.PerformanceCounters{
					Number:     1,
					Operations: 1,
					Size:       1,
					Errors:     1,
				},
				Timers: events.PerformanceTimers{
					Duration: time.Duration(1000),
					Total:    time.Duration(5000),
				},
				Gauges: events.PerformanceGauges{
					State:   1,
					Workers: 100,
					Failed:  true,
				},
			},
			expectedOutput: APIPerformanceEvent{
				Timestamp: NewTime(time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC)),
				Counters: APIPerformanceCounters{
					Number:     1,
					Operations: 1,
					Size:       1,
					Errors:     1,
				},
				Timers: APIPerformanceTimers{
					Duration: NewAPIDuration(time.Duration(1000)),
					Total:    NewAPIDuration(time.Duration(5000)),
				},
				Gauges: APIPerformanceGauges{
					State:   1,
					Workers: 100,
					Failed:  true,
				},
			},
		},
		{
			name: "GetPerfRollupValue",
			input: dbmodel.PerfRollupValue{
				Name:    "name",
				Value:   "value",
				Version: 1,
			},
			expectedOutput: APIPerfRollupValue{
				Name:    ToAPIString("name"),
				Value:   "value",
				Version: 1,
			},
		},
		{
			name: "GetPerfRollups",
			input: dbmodel.PerfRollups{
				Stats: []dbmodel.PerfRollupValue{
					dbmodel.PerfRollupValue{
						Name:    "stat0",
						Value:   "value0",
						Version: 1,
					},
					dbmodel.PerfRollupValue{
						Name:          "stat1",
						Value:         "value1",
						Version:       2,
						UserSubmitted: true,
					},
					dbmodel.PerfRollupValue{
						Name:    "stat2",
						Value:   "value2",
						Version: 3,
					},
				},
				ProcessedAt: time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC),
				Count:       3,
				Valid:       true,
			},
			expectedOutput: APIPerfRollups{
				Stats: []APIPerfRollupValue{
					APIPerfRollupValue{
						Name:    ToAPIString("stat0"),
						Value:   "value0",
						Version: 1,
					},
					APIPerfRollupValue{
						Name:          ToAPIString("stat1"),
						Value:         "value1",
						Version:       2,
						UserSubmitted: true,
					},
					APIPerfRollupValue{
						Name:    ToAPIString("stat2"),
						Value:   "value2",
						Version: 3,
					},
				},
				ProcessedAt: NewTime(time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC)),
				Count:       3,
				Valid:       true,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output interface{}
			switch i := test.input.(type) {
			case dbmodel.PerformanceResultInfo:
				output = getPerformanceResultInfo(i)
			case dbmodel.ArtifactInfo:
				output = getArtifactInfo(i)
			case *events.Performance:
				output = getPerformanceEvent(i)
			case dbmodel.PerfRollupValue:
				output = getPerfRollupValue(i)
			case dbmodel.PerfRollups:
				output = getPerfRollups(i)
			default:
				fmt.Println("no test, unknown type", i)
			}
			assert.Equal(t, test.expectedOutput, output)
		})
	}
}

func TestImport(t *testing.T) {
	for _, test := range []struct {
		name     string
		input    interface{}
		expected APIPerformanceResult
		err      bool
	}{
		{
			name: "TestImport",
			input: dbmodel.PerformanceResult{
				ID: "ID",
				Info: dbmodel.PerformanceResultInfo{
					Project:   "project",
					Version:   "version",
					TaskName:  "taskname",
					TaskID:    "taskid",
					Execution: 1,
					TestName:  "testname",
					Trial:     1,
					Parent:    "parent",
					Tags:      []string{"tag0", "tag1", "tag2"},
					Arguments: map[string]int32{
						"argument0": 0,
						"argument1": 1,
						"argument2": 2,
					},
					Schema: 1,
				},
				CreatedAt:   time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC),
				CompletedAt: time.Date(2018, time.December, 31, 23, 59, 59, 0, time.UTC),
				Version:     1,
				Artifacts: []dbmodel.ArtifactInfo{
					dbmodel.ArtifactInfo{
						Type:        dbmodel.PailS3,
						Bucket:      "bucket0",
						Path:        "path0",
						Format:      dbmodel.FileFTDC,
						Compression: dbmodel.FileZip,
						Schema:      dbmodel.SchemaRawEvents,
						Tags:        []string{"tag0", "tag1", "tag2"},
						CreatedAt:   time.Date(2018, time.December, 31, 23, 59, 59, 0, time.UTC),
					},
					dbmodel.ArtifactInfo{
						Type:        dbmodel.PailS3,
						Bucket:      "bucket1",
						Path:        "path1",
						Format:      dbmodel.FileFTDC,
						Compression: dbmodel.FileZip,
						Schema:      dbmodel.SchemaRawEvents,
						Tags:        []string{"tag0", "tag1", "tag2"},
						CreatedAt:   time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC),
					},
					dbmodel.ArtifactInfo{
						Type:        dbmodel.PailS3,
						Bucket:      "bucket2",
						Path:        "path2",
						Format:      dbmodel.FileFTDC,
						Compression: dbmodel.FileZip,
						Schema:      dbmodel.SchemaRawEvents,
						Tags:        []string{"tag0", "tag1", "tag2"},
						CreatedAt:   time.Date(2015, time.December, 31, 23, 59, 59, 0, time.UTC),
					},
				},
				Total: &events.Performance{
					Timestamp: time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC),
					Counters: events.PerformanceCounters{
						Number:     1,
						Operations: 1,
						Size:       1,
						Errors:     1,
					},
					Timers: events.PerformanceTimers{
						Duration: time.Duration(1000),
						Total:    time.Duration(5000),
					},
					Gauges: events.PerformanceGauges{
						Workers: 100,
					},
				},
				Rollups: dbmodel.PerfRollups{
					Stats: []dbmodel.PerfRollupValue{
						dbmodel.PerfRollupValue{
							Name:    "stat0",
							Value:   "value0",
							Version: 1,
						},
						dbmodel.PerfRollupValue{
							Name:    "stat1",
							Value:   "value1",
							Version: 2,
						},
						dbmodel.PerfRollupValue{
							Name:    "stat2",
							Value:   "value2",
							Version: 3,
						},
					},
					ProcessedAt: time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC),
					Count:       3,
					Valid:       true,
				},
			},
			expected: APIPerformanceResult{
				Name: ToAPIString("ID"),
				Info: APIPerformanceResultInfo{
					Project:   ToAPIString("project"),
					Version:   ToAPIString("version"),
					TaskName:  ToAPIString("taskname"),
					TaskID:    ToAPIString("taskid"),
					Execution: 1,
					TestName:  ToAPIString("testname"),
					Trial:     1,
					Parent:    ToAPIString("parent"),
					Tags:      []string{"tag0", "tag1", "tag2"},
					Arguments: map[string]int32{
						"argument0": 0,
						"argument1": 1,
						"argument2": 2,
					},
					Schema: 1,
				},
				CreatedAt:   NewTime(time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC)),
				CompletedAt: NewTime(time.Date(2018, time.December, 31, 23, 59, 59, 0, time.UTC)),
				Version:     1,
				Artifacts: []APIArtifactInfo{
					APIArtifactInfo{
						Type:        ToAPIString(string(dbmodel.PailS3)),
						Bucket:      ToAPIString("bucket0"),
						Path:        ToAPIString("path0"),
						Format:      ToAPIString(string(dbmodel.FileFTDC)),
						Compression: ToAPIString(string(dbmodel.FileZip)),
						Schema:      ToAPIString(string(dbmodel.SchemaRawEvents)),
						Tags:        []string{"tag0", "tag1", "tag2"},
						CreatedAt:   NewTime(time.Date(2018, time.December, 31, 23, 59, 59, 0, time.UTC)),
					},
					APIArtifactInfo{
						Type:        ToAPIString(string(dbmodel.PailS3)),
						Bucket:      ToAPIString("bucket1"),
						Path:        ToAPIString("path1"),
						Format:      ToAPIString(string(dbmodel.FileFTDC)),
						Compression: ToAPIString(string(dbmodel.FileZip)),
						Schema:      ToAPIString(string(dbmodel.SchemaRawEvents)),
						Tags:        []string{"tag0", "tag1", "tag2"},
						CreatedAt:   NewTime(time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC)),
					},
					APIArtifactInfo{
						Type:        ToAPIString(string(dbmodel.PailS3)),
						Bucket:      ToAPIString("bucket2"),
						Path:        ToAPIString("path2"),
						Format:      ToAPIString(string(dbmodel.FileFTDC)),
						Compression: ToAPIString(string(dbmodel.FileZip)),
						Schema:      ToAPIString(string(dbmodel.SchemaRawEvents)),
						Tags:        []string{"tag0", "tag1", "tag2"},
						CreatedAt:   NewTime(time.Date(2015, time.December, 31, 23, 59, 59, 0, time.UTC)),
					},
				},
				Total: &APIPerformanceEvent{
					Timestamp: NewTime(time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC)),
					Counters: APIPerformanceCounters{
						Number:     1,
						Operations: 1,
						Size:       1,
						Errors:     1,
					},
					Timers: APIPerformanceTimers{
						Duration: NewAPIDuration(time.Duration(1000)),
						Total:    NewAPIDuration(time.Duration(5000)),
					},
					Gauges: APIPerformanceGauges{
						Workers: 100,
					},
				},
				Rollups: APIPerfRollups{
					Stats: []APIPerfRollupValue{
						APIPerfRollupValue{
							Name:    ToAPIString("stat0"),
							Value:   "value0",
							Version: 1,
						},
						APIPerfRollupValue{
							Name:    ToAPIString("stat1"),
							Value:   "value1",
							Version: 2,
						},
						APIPerfRollupValue{
							Name:    ToAPIString("stat2"),
							Value:   "value2",
							Version: 3,
						},
					},
					ProcessedAt: NewTime(time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC)),
					Count:       3,
					Valid:       true,
				},
			},
		},
		{
			name:     "TestImportInvalidType",
			input:    APIPerformanceEvent{},
			expected: APIPerformanceResult{},
			err:      true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			apiPerformanceResult := APIPerformanceResult{}
			err := apiPerformanceResult.Import(test.input)
			assert.Equal(t, test.expected, apiPerformanceResult)
			if test.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}

}
