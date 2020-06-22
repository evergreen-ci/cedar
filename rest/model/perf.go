package model

import (
	dbmodel "github.com/evergreen-ci/cedar/model"

	"github.com/pkg/errors"
)

// APIPerformanceResult describes a single result of a performance test from
// Evergreen.
type APIPerformanceResult struct {
	Name        APIString                `json:"name"`
	Info        APIPerformanceResultInfo `json:"info"`
	CreatedAt   APITime                  `json:"created_at"`
	CompletedAt APITime                  `json:"completed_at"`
	Artifacts   []APIArtifactInfo        `json:"artifacts"`
	Rollups     APIPerfRollups           `json:"rollups"`
	Analysis    APIPerfAnalysis          `json:"analysis"`
}

type APIPerfAnalysis struct {
	ProcessedAt APITime `bson:"processed_at" json:"processed_at" yaml:"processed_at"`
}

// Import transforms a PerformanceResult object into an APIPerformanceResult
// object.
func (apiResult *APIPerformanceResult) Import(i interface{}) error {
	switch r := i.(type) {
	case dbmodel.PerformanceResult:
		apiResult.Name = ToAPIString(r.ID)
		apiResult.CreatedAt = NewTime(r.CreatedAt)
		apiResult.CompletedAt = NewTime(r.CompletedAt)
		apiResult.Info = getPerformanceResultInfo(r.Info)
		apiResult.Rollups = getPerfRollups(r.Rollups)
		apiResult.Analysis = getAnalysis(r.Analysis)

		var apiArtifacts []APIArtifactInfo
		for _, artifactInfo := range r.Artifacts {
			apiArtifacts = append(apiArtifacts, getArtifactInfo(artifactInfo))
		}
		apiResult.Artifacts = apiArtifacts
	default:
		return errors.New("incorrect type when fetching converting PerformanceResult type")
	}
	return nil
}

func (apiResult *APIPerformanceResult) Export(i interface{}) (interface{}, error) {
	return nil, errors.Errorf("Export is not implemented for APIPerformanceResult")
}

// APIPerformanceResultInfo describes information unique to a single
// performance result.
type APIPerformanceResultInfo struct {
	Project   APIString        `json:"project"`
	Version   APIString        `json:"version"`
	Order     int              `json:"order"`
	Variant   APIString        `json:"variant"`
	TaskName  APIString        `json:"task_name"`
	TaskID    APIString        `json:"task_id"`
	Execution int              `json:"execution"`
	TestName  APIString        `json:"test_name"`
	Trial     int              `json:"trial"`
	Parent    APIString        `json:"parent"`
	Tags      []string         `json:"tags"`
	Arguments map[string]int32 `json:"args"`
}

func getPerformanceResultInfo(r dbmodel.PerformanceResultInfo) APIPerformanceResultInfo {
	return APIPerformanceResultInfo{
		Project:   ToAPIString(r.Project),
		Version:   ToAPIString(r.Version),
		Order:     r.Order,
		Variant:   ToAPIString(r.Variant),
		TaskName:  ToAPIString(r.TaskName),
		TaskID:    ToAPIString(r.TaskID),
		Execution: r.Execution,
		TestName:  ToAPIString(r.TestName),
		Trial:     r.Trial,
		Parent:    ToAPIString(r.Parent),
		Tags:      r.Tags,
		Arguments: r.Arguments,
	}
}

// APIArtifactInfo is a type that describes an object in some kind of offline
// storage, and is the bridge between pail-backed offline-storage and the
// cedar-based metadata storage.
type APIArtifactInfo struct {
	Type        APIString `json:"type"`
	Bucket      APIString `json:"bucket"`
	Prefix      APIString `json:"prefix"`
	Path        APIString `json:"path"`
	Format      APIString `json:"format"`
	Compression APIString `json:"compression"`
	Schema      APIString `json:"schema"`
	Tags        []string  `json:"tags"`
	CreatedAt   APITime   `json:"created_at"`
	DownloadURL APIString `json:"download_url"`
}

func getArtifactInfo(r dbmodel.ArtifactInfo) APIArtifactInfo {
	return APIArtifactInfo{
		Type:        ToAPIString(string(r.Type)),
		Bucket:      ToAPIString(r.Bucket),
		Prefix:      ToAPIString(r.Prefix),
		Path:        ToAPIString(r.Path),
		Format:      ToAPIString(string(r.Format)),
		Compression: ToAPIString(string(r.Compression)),
		Schema:      ToAPIString(string(r.Schema)),
		Tags:        r.Tags,
		CreatedAt:   NewTime(r.CreatedAt),
		DownloadURL: ToAPIString(r.GetDownloadURL()),
	}
}

// APIPerfRollups describes the "rolled up", or calculated metrics from time
// series data collected in a given performance test, of a performance result.
type APIPerfRollups struct {
	Stats       []APIPerfRollupValue `json:"stats"`
	ProcessedAt APITime              `json:"processed_at"`
}

// APIPerfRollupValue describes a single "rollup", see APIPerfRollups for more
// information.
type APIPerfRollupValue struct {
	Name          APIString   `json:"name"`
	Value         interface{} `json:"val"`
	Version       int         `json:"version"`
	UserSubmitted bool        `json:"user"`
}

func getPerfRollups(r dbmodel.PerfRollups) APIPerfRollups {
	rollups := APIPerfRollups{
		ProcessedAt: NewTime(r.ProcessedAt),
	}

	var apiStats []APIPerfRollupValue
	for _, stat := range r.Stats {
		apiStats = append(apiStats, getPerfRollupValue(stat))
	}
	rollups.Stats = apiStats

	return rollups
}

func getAnalysis(r dbmodel.PerfAnalysis) APIPerfAnalysis {
	return APIPerfAnalysis{
		ProcessedAt: NewTime(r.ProcessedAt),
	}
}

func getPerfRollupValue(r dbmodel.PerfRollupValue) APIPerfRollupValue {
	return APIPerfRollupValue{
		Name:          ToAPIString(r.Name),
		Value:         r.Value,
		Version:       r.Version,
		UserSubmitted: r.UserSubmitted,
	}
}
