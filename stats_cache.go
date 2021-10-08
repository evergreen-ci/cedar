package cedar

import (
	"context"
	"fmt"
	"sort"

	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/sometimes"
)

const topN = 10
const statChanBufferSize = 1000

func newStatsCacheRegistry(ctx context.Context) map[string]*StatsCache {
	registry := map[string]*StatsCache{
		StatsCacheBuildlogger: newStatsCache(StatsCacheBuildlogger),
		StatsCacheTestResults: newStatsCache(StatsCacheTestResults),
		StatsCachePerf:        newStatsCache(StatsCachePerf),
	}
	for _, r := range registry {
		go r.startConsumerLoop(ctx)
	}

	return registry
}

type Stat struct {
	Count   int
	Project string
	Version string
	Task    string
}

type StatsCache struct {
	cacheName string
	statChan  chan Stat

	calls     int
	total     int
	byProject map[string]int
	byVersion map[string]int
	byTask    map[string]int
}

func newStatsCache(name string) *StatsCache {
	return &StatsCache{
		statChan:  make(chan Stat, statChanBufferSize),
		byProject: make(map[string]int),
		byVersion: make(map[string]int),
		byTask:    make(map[string]int),
	}
}

func (s *StatsCache) resetCache() {
	s.calls = 0
	s.total = 0
	s.byProject = make(map[string]int)
	s.byVersion = make(map[string]int)
	s.byTask = make(map[string]int)
}

func (s *StatsCache) cacheStat(newStat Stat) {
	s.calls++
	s.total += newStat.Count
	s.byProject[newStat.Project] += newStat.Count
	s.byVersion[newStat.Version] += newStat.Count
	s.byTask[newStat.Task] += newStat.Count
}

func (s *StatsCache) startConsumerLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case nextStat := <-s.statChan:
			s.cacheStat(nextStat)
		}
	}
}

func (s *StatsCache) LogStats() {
	grip.Info(message.Fields{
		"message":    fmt.Sprintf("%s stats", s.cacheName),
		"calls":      s.calls,
		"total":      s.total,
		"by_project": topNMap(s.byProject, topN),
		"by_version": topNMap(s.byVersion, topN),
		"by_task":    topNMap(s.byTask, topN),
	})

	s.resetCache()
}

func (s *StatsCache) AddStat(newStat Stat) {
	select {
	case s.statChan <- newStat:
	default:
		grip.InfoWhen(sometimes.Percent(10), message.Fields{
			"message": "stats were dropped",
			"cache":   s.cacheName,
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
