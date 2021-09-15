package model

import (
	"sync"

	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
)

var CacheRegistry []StatsCache

type StatsCache interface {
	LogStats()
}

var buildLoggerCache *buildloggerStatsCache
var testResultsCache *testResultsStatsCache
var perfCache *perfStatsCache

func init() {
	buildLoggerCache = newBuildLoggerStatsCache()
	testResultsCache = newTestResultsStatsCache()
	perfCache = newPerfStatsCacheStatsCache()

	CacheRegistry = append(CacheRegistry, buildLoggerCache)
	CacheRegistry = append(CacheRegistry, testResultsCache)
	CacheRegistry = append(CacheRegistry, perfCache)
}

type buildloggerStatsCache struct {
	Lock sync.Mutex

	TotalLines     int
	LinesByVersion map[string]int
	LinesByProject map[string]int
}

func newBuildLoggerStatsCache() *buildloggerStatsCache {
	return &buildloggerStatsCache{
		LinesByVersion: make(map[string]int),
		LinesByProject: make(map[string]int),
	}
}

func (b *buildloggerStatsCache) LogStats() {
	b.Lock.Lock()
	defer b.Lock.Unlock()

	grip.InfoWhen(b.TotalLines > 0, message.Fields{
		"message":          "buildlogger counts",
		"total_lines":      b.TotalLines,
		"lines_by_project": b.LinesByProject,
		"lines_by_version": b.LinesByVersion,
	})

	b.TotalLines = 0
	b.LinesByVersion = make(map[string]int)
	b.LinesByProject = make(map[string]int)
}

func (b *buildloggerStatsCache) addLogLinesCount(l *Log, count int) {
	b.Lock.Lock()
	defer b.Lock.Unlock()

	b.TotalLines += count
	b.LinesByProject[l.Info.Project] += count
	b.LinesByVersion[l.Info.Version] += count
}

type testResultsStatsCache struct {
	Lock sync.Mutex

	TotalResults     int
	ResultsByVersion map[string]int
	ResultsByProject map[string]int
}

func newTestResultsStatsCache() *testResultsStatsCache {
	return &testResultsStatsCache{
		ResultsByVersion: make(map[string]int),
		ResultsByProject: make(map[string]int),
	}
}

func (r *testResultsStatsCache) LogStats() {
	r.Lock.Lock()
	defer r.Lock.Unlock()

	grip.InfoWhen(r.TotalResults > 0, message.Fields{
		"message":            "test results counts",
		"total_results":      r.TotalResults,
		"results_by_project": r.ResultsByProject,
		"results_by_version": r.ResultsByVersion,
	})

	r.TotalResults = 0
	r.ResultsByVersion = make(map[string]int)
	r.ResultsByProject = make(map[string]int)
}

func (r *testResultsStatsCache) addResultsCount(t *TestResults, count int) {
	r.Lock.Lock()
	defer r.Lock.Unlock()

	r.TotalResults += count
	r.ResultsByProject[t.Info.Project] += count
	r.ResultsByVersion[t.Info.Version] += count
}

type perfStatsCache struct {
	Lock sync.Mutex

	TotalArtifacts     int
	ArtifactsByVersion map[string]int
	ArtifactsByProject map[string]int
}

func newPerfStatsCacheStatsCache() *perfStatsCache {
	return &perfStatsCache{
		ArtifactsByVersion: make(map[string]int),
		ArtifactsByProject: make(map[string]int),
	}
}

func (p *perfStatsCache) LogStats() {
	p.Lock.Lock()
	defer p.Lock.Unlock()

	grip.InfoWhen(p.TotalArtifacts > 0, message.Fields{
		"message":              "perf counts",
		"total_artifacts":      p.TotalArtifacts,
		"artifacts_by_project": p.ArtifactsByProject,
		"artifacts_by_version": p.ArtifactsByVersion,
	})

	p.TotalArtifacts = 0
	p.ArtifactsByVersion = make(map[string]int)
	p.ArtifactsByProject = make(map[string]int)
}

func (p *perfStatsCache) addArtifactsCount(r *PerformanceResult, count int) {
	p.Lock.Lock()
	defer p.Lock.Unlock()

	p.TotalArtifacts += count
	p.ArtifactsByVersion[r.Info.Project] += count
	p.ArtifactsByProject[r.Info.Version] += count
}
