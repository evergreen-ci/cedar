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

func init() {
	buildLoggerCache = &buildloggerStatsCache{
		LinesByVersion: make(map[string]int),
		LinesByProject: make(map[string]int),
	}

	CacheRegistry = append(CacheRegistry, buildLoggerCache)
}

type buildloggerStatsCache struct {
	Lock sync.Mutex

	TotalLines     int
	LinesByVersion map[string]int
	LinesByProject map[string]int
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

func (b *buildloggerStatsCache) addLogLinesInfo(l *Log, lineCount int) {
	b.Lock.Lock()
	defer b.Lock.Unlock()

	b.TotalLines += lineCount
	b.LinesByProject[l.Info.Project] += lineCount
	b.LinesByVersion[l.Info.Version] += lineCount
}
