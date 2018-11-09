package model

import (
	"fmt"
	"testing"
	"time"

	dbmodel "github.com/evergreen-ci/sink/model"
	"github.com/stretchr/testify/assert"
)

func TestImportHelperFunctions(t *testing.T) {
	for _, test := range []struct {
		name           string
		input          interface{}
		expectedOutput interface{}
	}{
		{
			name: "TestgetPerformanceResultID",
			input: dbmodel.PerformanceResultID{
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
			expectedOutput: APIPerformanceResultID{
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
			name: "TestgetArtifactInfo",
			input: dbmodel.ArtifactInfo{
				Type:        dbmodel.PailS3,
				Bucket:      "bucket",
				Path:        "path",
				Format:      dbmodel.FileFTDC,
				Compression: dbmodel.FileZip,
				Tags:        []string{"tag0", "tag1", "tag2"},
			},
			expectedOutput: APIArtifactInfo{
				Type:        ToAPIString(string(dbmodel.PailS3)),
				Bucket:      ToAPIString("bucket"),
				Path:        ToAPIString("path"),
				Format:      ToAPIString(string(dbmodel.FileFTDC)),
				Compression: ToAPIString(string(dbmodel.FileZip)),
				Tags:        []string{"tag0", "tag1", "tag2"},
			},
		},
		{
			name: "TestgetPerformancePoint",
			input: &dbmodel.PerformancePoint{
				Timestamp: time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC),
				Counters: struct {
					Operations int64 `bson:"ops" json:"ops" yaml:"ops"`
					Size       int64 `bson:"size" json:"size" yaml:"size"`
					Errors     int64 `bson:"errors" json:"errors" yaml:"errors"`
				}{
					Operations: 1,
					Size:       1,
					Errors:     1,
				},
				Timers: struct {
					Duration time.Duration `bson:"dur" json:"dur" yaml:"dur"`
					Waiting  time.Duration `bson:"wait" json:"wait" yaml:"wait"`
				}{
					Duration: time.Duration(1000),
					Waiting:  time.Duration(5000),
				},
				State: struct {
					Workers int64 `bson:"workers" json:"workers" yaml:"workers"`
					Failed  bool  `bson:"failed" json:"failed" yaml:"failed"`
				}{
					Workers: 100,
				},
			},
			expectedOutput: APIPerformancePoint{
				Timestamp: NewTime(time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC)),
				Counters: struct {
					Operations int64 `json:"ops"`
					Size       int64 `json:"size"`
					Errors     int64 `json:"errors"`
				}{
					Operations: 1,
					Size:       1,
					Errors:     1,
				},
				Timers: struct {
					Duration APIDuration `json:"dur"`
					Waiting  APIDuration `json:"wait"`
				}{
					Duration: NewAPIDuration(time.Duration(1000)),
					Waiting:  NewAPIDuration(time.Duration(5000)),
				},
				State: struct {
					Workers int64 `json:"workers"`
					Failed  bool  `json:"failed"`
				}{
					Workers: 100,
				},
			},
		},
		{
			name: "TestgetPerfRollupValue",
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
			name: "TestgetPerfRollups",
			input: &dbmodel.PerfRollups{
				DefaultStats: []dbmodel.PerfRollupValue{
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
				UserStats: []dbmodel.PerfRollupValue{
					dbmodel.PerfRollupValue{
						Name:    "user0",
						Value:   1,
						Version: 1,
					},
					dbmodel.PerfRollupValue{
						Name:    "user1",
						Value:   2,
						Version: 2,
					},
				},
				ProcessedAt: time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC),
				Count:       5,
				Valid:       true,
			},
			expectedOutput: APIPerfRollups{
				DefaultStats: []APIPerfRollupValue{
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
				UserStats: []APIPerfRollupValue{
					APIPerfRollupValue{
						Name:    ToAPIString("user0"),
						Value:   1,
						Version: 1,
					},
					APIPerfRollupValue{
						Name:    ToAPIString("user1"),
						Value:   2,
						Version: 2,
					},
				},
				ProcessedAt: NewTime(time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC)),
				Count:       5,
				Valid:       true,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output interface{}
			switch i := test.input.(type) {
			case dbmodel.PerformanceResultID:
				output = getPerformanceResultID(i)
			case dbmodel.ArtifactInfo:
				output = getArtifactInfo(i)
			case *dbmodel.PerformancePoint:
				output = getPerformancePoint(i)
			case dbmodel.PerfRollupValue:
				output = getPerfRollupValue(i)
			case *dbmodel.PerfRollups:
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
				Info: dbmodel.PerformanceResultID{
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
				Source: []dbmodel.ArtifactInfo{
					dbmodel.ArtifactInfo{
						Type:        dbmodel.PailS3,
						Bucket:      "bucket0",
						Path:        "path0",
						Format:      dbmodel.FileFTDC,
						Compression: dbmodel.FileZip,
						Tags:        []string{"tag0", "tag1", "tag2"},
					},
					dbmodel.ArtifactInfo{
						Type:        dbmodel.PailS3,
						Bucket:      "bucket1",
						Path:        "path1",
						Format:      dbmodel.FileFTDC,
						Compression: dbmodel.FileZip,
						Tags:        []string{"tag0", "tag1", "tag2"},
					},
					dbmodel.ArtifactInfo{
						Type:        dbmodel.PailS3,
						Bucket:      "bucket2",
						Path:        "path2",
						Format:      dbmodel.FileFTDC,
						Compression: dbmodel.FileZip,
						Tags:        []string{"tag0", "tag1", "tag2"},
					},
				},
				AuxilaryData: []dbmodel.ArtifactInfo{
					dbmodel.ArtifactInfo{
						Type:        dbmodel.PailLegacyGridFS,
						Bucket:      "bucket0",
						Path:        "path0",
						Format:      dbmodel.FileFTDC,
						Compression: dbmodel.FileZip,
						Tags:        []string{"tag0", "tag1"},
					},
					dbmodel.ArtifactInfo{
						Type:        dbmodel.PailLegacyGridFS,
						Bucket:      "bucket0",
						Path:        "path0",
						Format:      dbmodel.FileFTDC,
						Compression: dbmodel.FileZip,
						Tags:        []string{"tag0", "tag1"},
					},
				},
				Total: &dbmodel.PerformancePoint{
					Timestamp: time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC),
					Counters: struct {
						Operations int64 `bson:"ops" json:"ops" yaml:"ops"`
						Size       int64 `bson:"size" json:"size" yaml:"size"`
						Errors     int64 `bson:"errors" json:"errors" yaml:"errors"`
					}{
						Operations: 1,
						Size:       1,
						Errors:     1,
					},
					Timers: struct {
						Duration time.Duration `bson:"dur" json:"dur" yaml:"dur"`
						Waiting  time.Duration `bson:"wait" json:"wait" yaml:"wait"`
					}{
						Duration: time.Duration(1000),
						Waiting:  time.Duration(5000),
					},
					State: struct {
						Workers int64 `bson:"workers" json:"workers" yaml:"workers"`
						Failed  bool  `bson:"failed" json:"failed" yaml:"failed"`
					}{
						Workers: 100,
					},
				},
				Rollups: &dbmodel.PerfRollups{
					DefaultStats: []dbmodel.PerfRollupValue{
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
					UserStats: []dbmodel.PerfRollupValue{
						dbmodel.PerfRollupValue{
							Name:    "user0",
							Value:   1,
							Version: 1,
						},
						dbmodel.PerfRollupValue{
							Name:    "user1",
							Value:   2,
							Version: 2,
						},
					},
					ProcessedAt: time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC),
					Count:       5,
					Valid:       true,
				},
			},
			expected: APIPerformanceResult{
				Name: ToAPIString("ID"),
				Info: APIPerformanceResultID{
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
				Source: []APIArtifactInfo{
					APIArtifactInfo{
						Type:        ToAPIString(string(dbmodel.PailS3)),
						Bucket:      ToAPIString("bucket0"),
						Path:        ToAPIString("path0"),
						Format:      ToAPIString(string(dbmodel.FileFTDC)),
						Compression: ToAPIString(string(dbmodel.FileZip)),
						Tags:        []string{"tag0", "tag1", "tag2"},
					},
					APIArtifactInfo{
						Type:        ToAPIString(string(dbmodel.PailS3)),
						Bucket:      ToAPIString("bucket1"),
						Path:        ToAPIString("path1"),
						Format:      ToAPIString(string(dbmodel.FileFTDC)),
						Compression: ToAPIString(string(dbmodel.FileZip)),
						Tags:        []string{"tag0", "tag1", "tag2"},
					},
					APIArtifactInfo{
						Type:        ToAPIString(string(dbmodel.PailS3)),
						Bucket:      ToAPIString("bucket2"),
						Path:        ToAPIString("path2"),
						Format:      ToAPIString(string(dbmodel.FileFTDC)),
						Compression: ToAPIString(string(dbmodel.FileZip)),
						Tags:        []string{"tag0", "tag1", "tag2"},
					},
				},
				AuxilaryData: []APIArtifactInfo{
					APIArtifactInfo{
						Type:        ToAPIString(string(dbmodel.PailLegacyGridFS)),
						Bucket:      ToAPIString("bucket0"),
						Path:        ToAPIString("path0"),
						Format:      ToAPIString(string(dbmodel.FileFTDC)),
						Compression: ToAPIString(string(dbmodel.FileZip)),
						Tags:        []string{"tag0", "tag1"},
					},
					APIArtifactInfo{
						Type:        ToAPIString(string(dbmodel.PailLegacyGridFS)),
						Bucket:      ToAPIString("bucket0"),
						Path:        ToAPIString("path0"),
						Format:      ToAPIString(string(dbmodel.FileFTDC)),
						Compression: ToAPIString(string(dbmodel.FileZip)),
						Tags:        []string{"tag0", "tag1"},
					},
				},
				Total: &APIPerformancePoint{
					Timestamp: NewTime(time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC)),
					Counters: struct {
						Operations int64 `json:"ops"`
						Size       int64 `json:"size"`
						Errors     int64 `json:"errors"`
					}{
						Operations: 1,
						Size:       1,
						Errors:     1,
					},
					Timers: struct {
						Duration APIDuration `json:"dur"`
						Waiting  APIDuration `json:"wait"`
					}{
						Duration: NewAPIDuration(time.Duration(1000)),
						Waiting:  NewAPIDuration(time.Duration(5000)),
					},
					State: struct {
						Workers int64 `json:"workers"`
						Failed  bool  `json:"failed"`
					}{
						Workers: 100,
					},
				},
				Rollups: &APIPerfRollups{
					DefaultStats: []APIPerfRollupValue{
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
					UserStats: []APIPerfRollupValue{
						APIPerfRollupValue{
							Name:    ToAPIString("user0"),
							Value:   1,
							Version: 1,
						},
						APIPerfRollupValue{
							Name:    ToAPIString("user1"),
							Value:   2,
							Version: 2,
						},
					},
					ProcessedAt: NewTime(time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC)),
					Count:       5,
					Valid:       true,
				},
			},
		},
		{
			name:     "TestImportInvalidType",
			input:    APIPerformancePoint{},
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
