package cedar

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
)

const topN = 10
const statChanBufferSize = 1000

func newStatsCacheRegistry(ctx context.Context) map[string]*statsCache {
	registry := map[string]*statsCache{
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

type statsCache struct {
	cacheName string
	statChan  chan Stat

	calls     int
	total     int
	byProject map[string]int
	byVersion map[string]int
	byTask    map[string]int
}

func newStatsCache(name string) *statsCache {
	return &statsCache{
		statChan:  make(chan Stat, statChanBufferSize),
		byProject: make(map[string]int),
		byVersion: make(map[string]int),
		byTask:    make(map[string]int),
	}
}

func (s *statsCache) resetCache() {
	s.calls = 0
	s.total = 0
	s.byProject = make(map[string]int)
	s.byVersion = make(map[string]int)
	s.byTask = make(map[string]int)
}

func (s *statsCache) cacheStat(newStat Stat) {
	s.calls++
	s.total += newStat.Count
	s.byProject[newStat.Project] += newStat.Count
	s.byVersion[newStat.Version] += newStat.Count
	s.byTask[newStat.Task] += newStat.Count
}

func (s *statsCache) startConsumerLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case nextStat := <-s.statChan:
			s.cacheStat(nextStat)
		}
	}
}

func (s *statsCache) LogStats() {
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

func (s *statsCache) AddStat(newStat Stat) error {
	select {
	case s.statChan <- newStat:
		return nil
	default:
		return errors.New("stats cache is full")
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
