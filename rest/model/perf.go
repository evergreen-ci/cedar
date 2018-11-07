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

		apiInfo, err := getPerformanceResultID(r.Info)
		if err != nil {
			return errors.Wrap(err, "problem getting info")
		}
		apiResult.Info = apiInfo

		var apiSource []APIArtifactInfo
		for _, artifactInfo := range r.Source {
			apiArtifactInfo, err := getArtifactInfo(artifactInfo)
			if err != nil {
				return errors.Wrap(err, "problem getting source data")
			}
			apiSource = append(apiSource, apiArtifactInfo)
		}
		apiResult.Source = apiSource

		var apiAuxilaryData []APIArtifactInfo
		for _, artifactInfo := range r.AuxilaryData {
			apiArtifactInfo, err := getArtifactInfo(artifactInfo)
			if err != nil {
				return errors.Wrap(err, "problem getting auxilary data")
			}
			apiAuxilaryData = append(apiAuxilaryData, apiArtifactInfo)
		}
		apiResult.AuxilaryData = apiAuxilaryData

		total, err := getPerformancePoint(r.Total)
		if err != nil {
			return errors.Wrap(err, "problem getting total")
		}
		apiResult.Total = &total

		rollups, err := getPerfRollups(r.Rollups)
		if err != nil {
			return errors.Wrap(err, "problems getting rollups")
		}
		apiResult.Rollups = &rollups
	default:
		return errors.New("incorrect type when fetching converting PerformanceResult type")
	}
	return nil
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

func getPerformanceResultID(i interface{}) (APIPerformanceResultID, error) {
	switch r := i.(type) {
	case dbmodel.PerformanceResultID:
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
		}, nil
	default:
		return APIPerformanceResultID{}, errors.New("incorrect type when fetching converting PerformanceResultID type")
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

func getArtifactInfo(i interface{}) (APIArtifactInfo, error) {
	switch r := i.(type) {
	case dbmodel.ArtifactInfo:
		return APIArtifactInfo{
			Type:        ToAPIString(string(r.Type)),
			Bucket:      ToAPIString(r.Bucket),
			Path:        ToAPIString(r.Path),
			Format:      ToAPIString(string(r.Format)),
			Compression: ToAPIString(string(r.Compression)),
			Tags:        r.Tags,
		}, nil
	default:
		return APIArtifactInfo{}, errors.New("incorrect type when fetching ArtifactInfo type")
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

func getPerformancePoint(i interface{}) (APIPerformancePoint, error) {
	switch r := i.(type) {
	case *dbmodel.PerformancePoint:
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
		}, nil
	default:
		return APIPerformancePoint{}, errors.New("incorrect type when fetching PerformancePoint type")
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

func getPerfRollups(i interface{}) (APIPerfRollups, error) {
	switch r := i.(type) {
	case dbmodel.PerfRollups:
		rollups := APIPerfRollups{
			ProcessedAt: NewTime(r.ProcessedAt),
			Count:       r.Count,
			Valid:       r.Valid,
		}

		var apiDefaultStats []APIPerfRollupValue
		for _, defaultStat := range r.DefaultStats {
			apiDefaultStat, err := getPerfRollupValue(defaultStat)
			if err != nil {
				return APIPerfRollups{}, errors.Wrap(err, "problem getting PerfRollups type")
			}
			apiDefaultStats = append(apiDefaultStats, apiDefaultStat)
		}
		rollups.DefaultStats = apiDefaultStats

		var apiUserStats []APIPerfRollupValue
		for _, userStat := range r.UserStats {
			apiUserStat, err := getPerfRollupValue(userStat)
			if err != nil {
				return APIPerfRollups{}, errors.Wrap(err, "problem getting PerfRollups type")
			}
			apiUserStats = append(apiUserStats, apiUserStat)
		}
		rollups.UserStats = apiUserStats

		return rollups, nil
	default:
		return APIPerfRollups{}, errors.New("incorrect type when fetching PerfRollups type")
	}
}

func getPerfRollupValue(i interface{}) (APIPerfRollupValue, error) {
	switch r := i.(type) {
	case dbmodel.PerfRollupValue:
		return APIPerfRollupValue{
			Name:    ToAPIString(r.Name),
			Value:   r.Value,
			Version: r.Version,
		}, nil
	default:
		return APIPerfRollupValue{}, errors.New("incorrect type when fetching PerfRollupValue type")
	}
}
