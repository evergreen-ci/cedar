package model

import (
	"context"
	"sort"

	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/sometimes"
)

// StatsCache aggregates and logs stats about a service
type StatsCache interface {
	LogStats()
}

const topN = 10
const statChanBufferSize = 1000

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
	perfCache = newPerfStatsCache()

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
		statChan:  make(chan stat, statChanBufferSize),
		byProject: make(map[string]int),
		byVersion: make(map[string]int),
		byTask:    make(map[string]int),
	}
}

func (b *baseCache) resetCache() {
	b.calls = 0
	b.total = 0
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
	baseCache
}

func newTestResultsStatsCache() *testResultsStatsCache {
	return &testResultsStatsCache{
		baseCache: newBaseCache(),
	}
}

// LogStats logs a message with buildlogger stats
func (b *testResultsStatsCache) LogStats() {
	b.logStats("test results counts")
}

func (b *testResultsStatsCache) addResultsCount(t *TestResults, count int) {
	newStat := stat{
		count:   count,
		project: t.Info.Project,
		version: t.Info.Version,
		task:    t.Info.TaskID,
	}

	select {
	case b.statChan <- newStat:
	default:
		grip.InfoWhen(sometimes.Percent(10), message.Fields{
			"message": "stats were dropped",
			"cache":   "test results",
			"cause":   "stats cache is full",
		})
	}
}

type perfStatsCache struct {
	baseCache
}

func newPerfStatsCache() *perfStatsCache {
	return &perfStatsCache{
		baseCache: newBaseCache(),
	}
}

// LogStats logs a message with buildlogger stats
func (b *perfStatsCache) LogStats() {
	b.logStats("perf counts")
}

func (b *perfStatsCache) addArtifactsCount(r *PerformanceResult, count int) {
	newStat := stat{
		count:   count,
		project: r.Info.Project,
		version: r.Info.Version,
		task:    r.Info.TaskID,
	}

	select {
	case b.statChan <- newStat:
	default:
		grip.InfoWhen(sometimes.Percent(10), message.Fields{
			"message": "stats were dropped",
			"cache":   "perf",
			"cause":   "stats cache is full",
		})
	}
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
