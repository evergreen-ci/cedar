package model

import (
	"fmt"
	"testing"
	"time"

	dbmodel "github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/utility"
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
				Order:     1,
				TaskName:  "taskname",
				TaskID:    "taskid",
				Execution: 1,
				TestName:  "testname",
				Variant:   "foo",
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
				Project:   utility.ToStringPtr("project"),
				Version:   utility.ToStringPtr("version"),
				Order:     1,
				TaskName:  utility.ToStringPtr("taskname"),
				TaskID:    utility.ToStringPtr("taskid"),
				Execution: 1,
				TestName:  utility.ToStringPtr("testname"),
				Variant:   utility.ToStringPtr("foo"),
				Trial:     1,
				Parent:    utility.ToStringPtr("parent"),
				Tags:      []string{"tag0", "tag1", "tag2"},
				Arguments: map[string]int32{
					"argument0": 0,
					"argument1": 1,
					"argument2": 2,
				},
			},
		},
		{
			name: "GetArtifactInfo",
			input: dbmodel.ArtifactInfo{
				Type:        dbmodel.PailS3,
				Bucket:      "bucket",
				Prefix:      "prefix",
				Path:        "path",
				Format:      dbmodel.FileFTDC,
				Compression: dbmodel.FileZip,
				Schema:      dbmodel.SchemaCollapsedEvents,
				Tags:        []string{"tag0", "tag1", "tag2"},
				CreatedAt:   time.Date(2018, time.December, 31, 23, 59, 59, 0, time.UTC),
			},
			expectedOutput: &APIArtifactInfo{
				Type:        utility.ToStringPtr(string(dbmodel.PailS3)),
				Bucket:      utility.ToStringPtr("bucket"),
				Prefix:      utility.ToStringPtr("prefix"),
				Path:        utility.ToStringPtr("path"),
				Format:      utility.ToStringPtr(string(dbmodel.FileFTDC)),
				Compression: utility.ToStringPtr(string(dbmodel.FileZip)),
				Schema:      utility.ToStringPtr(string(dbmodel.SchemaCollapsedEvents)),
				Tags:        []string{"tag0", "tag1", "tag2"},
				CreatedAt:   NewTime(time.Date(2018, time.December, 31, 23, 59, 59, 0, time.UTC)),
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
				Name:    utility.ToStringPtr("name"),
				Value:   "value",
				Version: 1,
			},
		},
		{
			name: "GetPerfRollups",
			input: dbmodel.PerfRollups{
				Stats: []dbmodel.PerfRollupValue{
					{
						Name:    "stat0",
						Value:   "value0",
						Version: 1,
					},
					{
						Name:          "stat1",
						Value:         "value1",
						Version:       2,
						UserSubmitted: true,
					},
					{
						Name:    "stat2",
						Value:   "value2",
						Version: 3,
					},
				},
				ProcessedAt: time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC),
			},
			expectedOutput: APIPerfRollups{
				Stats: []APIPerfRollupValue{
					{
						Name:    utility.ToStringPtr("stat0"),
						Value:   "value0",
						Version: 1,
					},
					{
						Name:          utility.ToStringPtr("stat1"),
						Value:         "value1",
						Version:       2,
						UserSubmitted: true,
					},
					{
						Name:    utility.ToStringPtr("stat2"),
						Value:   "value2",
						Version: 3,
					},
				},
				ProcessedAt: NewTime(time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC)),
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
				expected := test.expectedOutput.(*APIArtifactInfo)
				expected.DownloadURL = utility.ToStringPtr(i.GetDownloadURL())
				test.expectedOutput = *expected
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
					Order:     1,
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
					{
						Type:        dbmodel.PailS3,
						Bucket:      "bucket0",
						Path:        "path0",
						Format:      dbmodel.FileFTDC,
						Compression: dbmodel.FileZip,
						Schema:      dbmodel.SchemaRawEvents,
						Tags:        []string{"tag0", "tag1", "tag2"},
						CreatedAt:   time.Date(2018, time.December, 31, 23, 59, 59, 0, time.UTC),
					},
					{
						Type:        dbmodel.PailS3,
						Bucket:      "bucket1",
						Prefix:      "prefix1",
						Path:        "path1",
						Format:      dbmodel.FileFTDC,
						Compression: dbmodel.FileZip,
						Schema:      dbmodel.SchemaRawEvents,
						Tags:        []string{"tag0", "tag1", "tag2"},
						CreatedAt:   time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC),
					},
					{
						Type:        dbmodel.PailS3,
						Bucket:      "bucket2",
						Prefix:      "prefix2",
						Path:        "path2",
						Format:      dbmodel.FileFTDC,
						Compression: dbmodel.FileZip,
						Schema:      dbmodel.SchemaRawEvents,
						Tags:        []string{"tag0", "tag1", "tag2"},
						CreatedAt:   time.Date(2015, time.December, 31, 23, 59, 59, 0, time.UTC),
					},
				},
				Rollups: dbmodel.PerfRollups{
					Stats: []dbmodel.PerfRollupValue{
						{
							Name:    "stat0",
							Value:   "value0",
							Version: 1,
						},
						{
							Name:    "stat1",
							Value:   "value1",
							Version: 2,
						},
						{
							Name:    "stat2",
							Value:   "value2",
							Version: 3,
						},
					},
					ProcessedAt: time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC),
				},
				Analysis: dbmodel.PerfAnalysis{
					ProcessedAt: time.Date(2015, time.December, 31, 23, 59, 59, 0, time.UTC),
				},
			},
			expected: APIPerformanceResult{
				Name: utility.ToStringPtr("ID"),
				Info: APIPerformanceResultInfo{
					Project:   utility.ToStringPtr("project"),
					Version:   utility.ToStringPtr("version"),
					Order:     1,
					TaskName:  utility.ToStringPtr("taskname"),
					TaskID:    utility.ToStringPtr("taskid"),
					Execution: 1,
					TestName:  utility.ToStringPtr("testname"),
					Trial:     1,
					Variant:   utility.ToStringPtr(""),
					Parent:    utility.ToStringPtr("parent"),
					Tags:      []string{"tag0", "tag1", "tag2"},
					Arguments: map[string]int32{
						"argument0": 0,
						"argument1": 1,
						"argument2": 2,
					},
				},
				CreatedAt:   NewTime(time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC)),
				CompletedAt: NewTime(time.Date(2018, time.December, 31, 23, 59, 59, 0, time.UTC)),
				Artifacts: []APIArtifactInfo{
					{
						Type:        utility.ToStringPtr(string(dbmodel.PailS3)),
						Bucket:      utility.ToStringPtr("bucket0"),
						Prefix:      utility.ToStringPtr(""),
						Path:        utility.ToStringPtr("path0"),
						Format:      utility.ToStringPtr(string(dbmodel.FileFTDC)),
						Compression: utility.ToStringPtr(string(dbmodel.FileZip)),
						Schema:      utility.ToStringPtr(string(dbmodel.SchemaRawEvents)),
						Tags:        []string{"tag0", "tag1", "tag2"},
						CreatedAt:   NewTime(time.Date(2018, time.December, 31, 23, 59, 59, 0, time.UTC)),
					},
					{
						Type:        utility.ToStringPtr(string(dbmodel.PailS3)),
						Bucket:      utility.ToStringPtr("bucket1"),
						Prefix:      utility.ToStringPtr("prefix1"),
						Path:        utility.ToStringPtr("path1"),
						Format:      utility.ToStringPtr(string(dbmodel.FileFTDC)),
						Compression: utility.ToStringPtr(string(dbmodel.FileZip)),
						Schema:      utility.ToStringPtr(string(dbmodel.SchemaRawEvents)),
						Tags:        []string{"tag0", "tag1", "tag2"},
						CreatedAt:   NewTime(time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC)),
					},
					{
						Type:        utility.ToStringPtr(string(dbmodel.PailS3)),
						Bucket:      utility.ToStringPtr("bucket2"),
						Prefix:      utility.ToStringPtr("prefix2"),
						Path:        utility.ToStringPtr("path2"),
						Format:      utility.ToStringPtr(string(dbmodel.FileFTDC)),
						Compression: utility.ToStringPtr(string(dbmodel.FileZip)),
						Schema:      utility.ToStringPtr(string(dbmodel.SchemaRawEvents)),
						Tags:        []string{"tag0", "tag1", "tag2"},
						CreatedAt:   NewTime(time.Date(2015, time.December, 31, 23, 59, 59, 0, time.UTC)),
					},
				},
				Rollups: APIPerfRollups{
					Stats: []APIPerfRollupValue{
						{
							Name:    utility.ToStringPtr("stat0"),
							Value:   "value0",
							Version: 1,
						},
						{
							Name:    utility.ToStringPtr("stat1"),
							Value:   "value1",
							Version: 2,
						},
						{
							Name:    utility.ToStringPtr("stat2"),
							Value:   "value2",
							Version: 3,
						},
					},
					ProcessedAt: NewTime(time.Date(2012, time.December, 31, 23, 59, 59, 0, time.UTC)),
				},
				Analysis: APIPerfAnalysis{
					ProcessedAt: NewTime(time.Date(2015, time.December, 31, 23, 59, 59, 0, time.UTC)),
				},
			},
		},
		{
			name:     "TestImportInvalidType",
			input:    APIArtifactInfo{},
			expected: APIPerformanceResult{},
			err:      true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			apiPerformanceResult := APIPerformanceResult{}
			err := apiPerformanceResult.Import(test.input)
			if test.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				for i := range test.expected.Artifacts {
					test.expected.Artifacts[i].DownloadURL = utility.ToStringPtr(test.input.(dbmodel.PerformanceResult).Artifacts[i].GetDownloadURL())
				}
			}
			assert.Equal(t, test.expected, apiPerformanceResult)
		})
	}

}
