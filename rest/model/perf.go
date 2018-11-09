package model

import (
	dbmodel "github.com/evergreen-ci/sink/model"
	"github.com/pkg/errors"
)

type APIPerformanceResult struct {
	Name         APIString              `json:"name"`
	Info         APIPerformanceResultID `json:"info"`
	CreatedAt    APITime                `json:"create_at"`
	CompletedAt  APITime                `json:"completed_at"`
	Version      int                    `json:"version"`
	Source       []APIArtifactInfo      `json:"source_info"`
	AuxilaryData []APIArtifactInfo      `json:aux_data"`
	Total        *APIPerformancePoint   `json:"total"`
	Rollups      *APIPerfRollups        `json:"rollups"`
}

func (apiResult *APIPerformanceResult) Import(i interface{}) error {
	switch r := i.(type) {
	case dbmodel.PerformanceResult:
		apiResult.Name = ToAPIString(r.ID)
		apiResult.CreatedAt = NewTime(r.CreatedAt)
		apiResult.CompletedAt = NewTime(r.CompletedAt)
		apiResult.Version = r.Version
		apiResult.Info = getPerformanceResultID(r.Info)

		total := getPerformancePoint(r.Total)
		apiResult.Total = &total
		rollups := getPerfRollups(r.Rollups)
		apiResult.Rollups = &rollups

		var apiSource []APIArtifactInfo
		for _, artifactInfo := range r.Source {
			apiArtifactInfo := getArtifactInfo(artifactInfo)
			apiSource = append(apiSource, apiArtifactInfo)
		}
		apiResult.Source = apiSource

		var apiAuxilaryData []APIArtifactInfo
		for _, artifactInfo := range r.AuxilaryData {
			apiArtifactInfo := getArtifactInfo(artifactInfo)
			apiAuxilaryData = append(apiAuxilaryData, apiArtifactInfo)
		}
		apiResult.AuxilaryData = apiAuxilaryData
	default:
		return errors.New("incorrect type when fetching converting PerformanceResult type")
	}
	return nil
}

func (apiResult *APIPerformanceResult) Export(i interface{}) (interface{}, error) {
	return nil, errors.Errorf("Export is not implemented for APIPerformanceResult")
}

type APIPerformanceResultID struct {
	Project   APIString        `json:"project"`
	Version   APIString        `json:"version"`
	TaskName  APIString        `json:"task_name"`
	TaskID    APIString        `json:"task_id"`
	Execution int              `json:"execution"`
	TestName  APIString        `json:"test_name"`
	Trial     int              `json:"trial"`
	Parent    APIString        `json:"parent"`
	Tags      []string         `json:"tags"`
	Arguments map[string]int32 `json:"args"`
	Schema    int              `json:"schema"`
}

func getPerformanceResultID(r dbmodel.PerformanceResultID) APIPerformanceResultID {
	return APIPerformanceResultID{
		Project:   ToAPIString(r.Project),
		Version:   ToAPIString(r.Version),
		TaskName:  ToAPIString(r.TaskName),
		TaskID:    ToAPIString(r.TaskID),
		Execution: r.Execution,
		TestName:  ToAPIString(r.TestName),
		Trial:     r.Trial,
		Parent:    ToAPIString(r.Parent),
		Tags:      r.Tags,
		Arguments: r.Arguments,
		Schema:    r.Schema,
	}
}

type APIArtifactInfo struct {
	Type        APIString `json:"type"`
	Bucket      APIString `json:"bucket"`
	Path        APIString `json:"path"`
	Format      APIString `json:"format"`
	Compression APIString `json:"compression"`
	Tags        []string  `json:"tags"`
}

func getArtifactInfo(r dbmodel.ArtifactInfo) APIArtifactInfo {
	return APIArtifactInfo{
		Type:        ToAPIString(string(r.Type)),
		Bucket:      ToAPIString(r.Bucket),
		Path:        ToAPIString(r.Path),
		Format:      ToAPIString(string(r.Format)),
		Compression: ToAPIString(string(r.Compression)),
		Tags:        r.Tags,
	}
}

type APIPerformancePoint struct {
	Timestamp APITime `json:"ts"`

	Counters struct {
		Operations int64 `json:"ops"`
		Size       int64 `json:"size"`
		Errors     int64 `json:"errors"`
	} `json:"counters"`

	Timers struct {
		Duration APIDuration `json:"dur"`
		Waiting  APIDuration `json:"wait"`
	} `json:"timers"`

	State struct {
		Workers int64 `json:"workers"`
		Failed  bool  `json:"failed"`
	} `json:"state"`
}

func getPerformancePoint(r *dbmodel.PerformancePoint) APIPerformancePoint {
	return APIPerformancePoint{
		Timestamp: NewTime(r.Timestamp),
		Counters: struct {
			Operations int64 `json:"ops"`
			Size       int64 `json:"size"`
			Errors     int64 `json:"errors"`
		}{
			Operations: r.Counters.Operations,
			Size:       r.Counters.Size,
			Errors:     r.Counters.Errors,
		},
		Timers: struct {
			Duration APIDuration `json:"dur"`
			Waiting  APIDuration `json:"wait"`
		}{
			Duration: NewAPIDuration(r.Timers.Duration),
			Waiting:  NewAPIDuration(r.Timers.Waiting),
		},
		State: struct {
			Workers int64 `json:"workers"`
			Failed  bool  `json:"failed"`
		}{
			Workers: r.State.Workers,
			Failed:  r.State.Failed,
		},
	}
}

type APIPerfRollups struct {
	DefaultStats []APIPerfRollupValue `json:"system"`
	UserStats    []APIPerfRollupValue `json:"user"`
	ProcessedAt  APITime              `json:"processed_at"`
	Count        int                  `json:"count"`
	Valid        bool                 `json:"valid"`
}

type APIPerfRollupValue struct {
	Name    APIString   `json:"name"`
	Value   interface{} `json:"val"`
	Version int         `json:"version"`
}

func getPerfRollups(r *dbmodel.PerfRollups) APIPerfRollups {
	rollups := APIPerfRollups{
		ProcessedAt: NewTime(r.ProcessedAt),
		Count:       r.Count,
		Valid:       r.Valid,
	}

	var apiDefaultStats []APIPerfRollupValue
	for _, defaultStat := range r.DefaultStats {
		apiDefaultStat := getPerfRollupValue(defaultStat)
		apiDefaultStats = append(apiDefaultStats, apiDefaultStat)
	}
	rollups.DefaultStats = apiDefaultStats

	var apiUserStats []APIPerfRollupValue
	for _, userStat := range r.UserStats {
		apiUserStat := getPerfRollupValue(userStat)
		apiUserStats = append(apiUserStats, apiUserStat)
	}
	rollups.UserStats = apiUserStats

	return rollups
}

func getPerfRollupValue(r dbmodel.PerfRollupValue) APIPerfRollupValue {
	return APIPerfRollupValue{
		Name:    ToAPIString(r.Name),
		Value:   r.Value,
		Version: r.Version,
	}
}
