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

func init() {
	buildLoggerCache = newBuildLoggerStatsCache()
	testResultsCache = newTestResultsStatsCache()

	CacheRegistry = append(CacheRegistry, buildLoggerCache)
	CacheRegistry = append(CacheRegistry, testResultsCache)
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

func (b *testResultsStatsCache) LogStats() {
	b.Lock.Lock()
	defer b.Lock.Unlock()

	grip.InfoWhen(b.TotalResults > 0, message.Fields{
		"message":            "test results counts",
		"total_results":      b.TotalResults,
		"results_by_project": b.ResultsByProject,
		"results_by_version": b.ResultsByVersion,
	})

	b.TotalResults = 0
	b.ResultsByVersion = make(map[string]int)
	b.ResultsByProject = make(map[string]int)
}

func (b *testResultsStatsCache) addResultsCount(t *TestResults, count int) {
	b.Lock.Lock()
	defer b.Lock.Unlock()

	b.TotalResults += count
	b.ResultsByProject[t.Info.Project] += count
	b.ResultsByVersion[t.Info.Version] += count
}
