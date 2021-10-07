package model

import (
	"context"
	"sort"
	"sync"

	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/sometimes"
)

// StatsCache aggregates and logs stats about a service
type StatsCache interface {
	LogStats()
}

const topN = 10
const statChanBuffer = 1000

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

type stat struct {
	count   int
	project string
	version string
	task    string
}

type baseCache struct {
	statChan chan stat

	calls     int
	total     int
	byProject map[string]int
	byVersion map[string]int
	byTask    map[string]int
}

func newBaseCache() baseCache {
	return baseCache{
		statChan:  make(chan stat, statChanBuffer),
		byProject: make(map[string]int),
		byVersion: make(map[string]int),
		byTask:    make(map[string]int),
	}
}

func (b *baseCache) resetCache() {
	b.byProject = make(map[string]int)
	b.byVersion = make(map[string]int)
	b.byTask = make(map[string]int)
}

func (b *baseCache) addStat(s stat) {
	b.calls++
	b.total += s.count
	b.byProject[s.project] += s.count
	b.byVersion[s.version] += s.count
	b.byTask[s.task] += s.count
}

func (b *baseCache) logStats(m string) {
	grip.Info(message.Fields{
		"message":    m,
		"calls":      b.calls,
		"total":      b.total,
		"by_project": topNMap(b.byProject, topN),
		"by_version": topNMap(b.byVersion, topN),
		"by_task":    topNMap(b.byTask, topN),
	})

	b.resetCache()
}

func (b *baseCache) consumerLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case nextStat := <-b.statChan:
			b.addStat(nextStat)
		}
	}
}

type buildloggerStatsCache struct {
	baseCache
}

func newBuildLoggerStatsCache() *buildloggerStatsCache {
	return &buildloggerStatsCache{
		baseCache: newBaseCache(),
	}
}

// LogStats logs a message with buildlogger stats
func (b *buildloggerStatsCache) LogStats() {
	b.logStats("buildlogger counts")
}

func (b *buildloggerStatsCache) addLogLinesCount(l *Log, count int) {
	newStat := stat{
		count:   count,
		project: l.Info.Project,
		version: l.Info.Version,
		task:    l.Info.TaskID,
	}

	select {
	case b.statChan <- newStat:
	default:
		grip.InfoWhen(sometimes.Percent(10), message.Fields{
			"message": "stats were dropped",
			"cache":   "buildlogger",
			"cause":   "stats cache is full",
		})
	}
}

type testResultsStatsCache struct {
	mu sync.Mutex

	totalCalls       int
	totalResults     int
	resultsByVersion map[string]int
	resultsByProject map[string]int
	resultsByTask    map[string]int
}

func newTestResultsStatsCache() *testResultsStatsCache {
	return &testResultsStatsCache{
		resultsByVersion: make(map[string]int),
		resultsByProject: make(map[string]int),
		resultsByTask:    make(map[string]int),
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
		"results_by_task":    topNMap(r.resultsByTask, topN),
	})

	r.totalCalls = 0
	r.totalResults = 0
	r.resultsByVersion = make(map[string]int)
	r.resultsByProject = make(map[string]int)
	r.resultsByTask = make(map[string]int)
}

func (r *testResultsStatsCache) addResultsCount(t *TestResults, count int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.totalCalls++
	r.totalResults += count
	r.resultsByProject[t.Info.Project] += count
	r.resultsByVersion[t.Info.Version] += count
	r.resultsByTask[t.Info.TaskID] += count
}

type perfStatsCache struct {
	mu sync.Mutex

	totalCalls         int
	totalArtifacts     int
	artifactsByVersion map[string]int
	artifactsByProject map[string]int
	artifactsByTask    map[string]int
}

func newPerfStatsCacheStatsCache() *perfStatsCache {
	return &perfStatsCache{
		artifactsByVersion: make(map[string]int),
		artifactsByProject: make(map[string]int),
		artifactsByTask:    make(map[string]int),
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
		"artifacts_by_task":    topNMap(p.artifactsByTask, topN),
	})

	p.totalCalls = 0
	p.totalArtifacts = 0
	p.artifactsByVersion = make(map[string]int)
	p.artifactsByProject = make(map[string]int)
	p.artifactsByTask = make(map[string]int)
}

func (p *perfStatsCache) addArtifactsCount(r *PerformanceResult, count int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.totalCalls++
	p.totalArtifacts += count
	p.artifactsByVersion[r.Info.Project] += count
	p.artifactsByProject[r.Info.Version] += count
	p.artifactsByTask[r.Info.TaskID] += count
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
