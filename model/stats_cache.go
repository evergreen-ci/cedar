package model

import (
	"sort"
	"sync"

	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
)

// StatsCache aggregates and logs stats about a service
type StatsCache interface {
	LogStats()
}

const topN = 10

var (
	// CacheRegistry holds instances of each of the stats caches in-memory
	CacheRegistry []StatsCache

	buildLoggerCache *buildloggerStatsCache
	testResultsCache *testResultsStatsCache
	perfCache        *perfStatsCache
)

func init() {
	buildLoggerCache = newBuildLoggerStatsCache()
	testResultsCache = newTestResultsStatsCache()
	perfCache = newPerfStatsCacheStatsCache()

	CacheRegistry = append(CacheRegistry, buildLoggerCache)
	CacheRegistry = append(CacheRegistry, testResultsCache)
	CacheRegistry = append(CacheRegistry, perfCache)
}

type buildloggerStatsCache struct {
	mu sync.Mutex

	totalCalls     int
	totalLines     int
	linesByVersion map[string]int
	linesByProject map[string]int
}

func newBuildLoggerStatsCache() *buildloggerStatsCache {
	return &buildloggerStatsCache{
		linesByVersion: make(map[string]int),
		linesByProject: make(map[string]int),
	}
}

// LogStats logs a message with buildlogger stats
func (b *buildloggerStatsCache) LogStats() {
	b.mu.Lock()
	defer b.mu.Unlock()

	grip.Info(message.Fields{
		"message":          "buildlogger counts",
		"total_calls":      b.totalCalls,
		"total_lines":      b.totalLines,
		"lines_by_project": topNMap(b.linesByProject, topN),
		"lines_by_version": topNMap(b.linesByVersion, topN),
	})

	b.totalCalls = 0
	b.totalLines = 0
	b.linesByVersion = make(map[string]int)
	b.linesByProject = make(map[string]int)
}

func (b *buildloggerStatsCache) addLogLinesCount(l *Log, count int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.totalCalls++
	b.totalLines += count
	b.linesByProject[l.Info.Project] += count
	b.linesByVersion[l.Info.Version] += count
}

type testResultsStatsCache struct {
	mu sync.Mutex

	totalCalls       int
	totalResults     int
	resultsByVersion map[string]int
	resultsByProject map[string]int
}

func newTestResultsStatsCache() *testResultsStatsCache {
	return &testResultsStatsCache{
		resultsByVersion: make(map[string]int),
		resultsByProject: make(map[string]int),
	}
}

// LogStats logs a message with test results stats
func (r *testResultsStatsCache) LogStats() {
	r.mu.Lock()
	defer r.mu.Unlock()

	grip.Info(message.Fields{
		"message":            "test results counts",
		"total_calls":        r.totalCalls,
		"total_results":      r.totalResults,
		"results_by_project": topNMap(r.resultsByProject, topN),
		"results_by_version": topNMap(r.resultsByVersion, topN),
	})

	r.totalCalls = 0
	r.totalResults = 0
	r.resultsByVersion = make(map[string]int)
	r.resultsByProject = make(map[string]int)
}

func (r *testResultsStatsCache) addResultsCount(t *TestResults, count int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.totalCalls++
	r.totalResults += count
	r.resultsByProject[t.Info.Project] += count
	r.resultsByVersion[t.Info.Version] += count
}

type perfStatsCache struct {
	mu sync.Mutex

	totalCalls         int
	totalArtifacts     int
	artifactsByVersion map[string]int
	artifactsByProject map[string]int
}

func newPerfStatsCacheStatsCache() *perfStatsCache {
	return &perfStatsCache{
		artifactsByVersion: make(map[string]int),
		artifactsByProject: make(map[string]int),
	}
}

// LogStats logs a message with perf stats
func (p *perfStatsCache) LogStats() {
	p.mu.Lock()
	defer p.mu.Unlock()

	grip.Info(message.Fields{
		"message":              "perf counts",
		"total_calls":          p.totalCalls,
		"total_artifacts":      p.totalArtifacts,
		"artifacts_by_project": topNMap(p.artifactsByProject, topN),
		"artifacts_by_version": topNMap(p.artifactsByVersion, topN),
	})

	p.totalCalls = 0
	p.totalArtifacts = 0
	p.artifactsByVersion = make(map[string]int)
	p.artifactsByProject = make(map[string]int)
}

func (p *perfStatsCache) addArtifactsCount(r *PerformanceResult, count int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.totalCalls++
	p.totalArtifacts += count
	p.artifactsByVersion[r.Info.Project] += count
	p.artifactsByProject[r.Info.Version] += count
}

func topNMap(fullMap map[string]int, n int) map[string]int {
	type item struct {
		identifier string
		count      int
	}
	items := make([]item, 0, len(fullMap))
	for identifier, count := range fullMap {
		items = append(items, item{identifier: identifier, count: count})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].count > items[j].count })

	result := make(map[string]int, n)
	for i, item := range items {
		if i >= n {
			break
		}
		result[item.identifier] = item.count
	}

	return result
}
