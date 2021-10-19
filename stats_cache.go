package cedar

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/recovery"
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
		go r.startLoggerLoop(ctx)
	}

	return registry
}

// Stat represents a count to add to the cache for a particular project/version/taskID combination.
type Stat struct {
	Count   int
	Project string
	Version string
	TaskID  string
}

type statsCache struct {
	cacheName string
	statChan  chan Stat

	calls     int
	total     int
	byProject map[string]int
	byVersion map[string]int
	byTaskID  map[string]int
}

func newStatsCache(name string) *statsCache {
	return &statsCache{
		statChan:  make(chan Stat, statChanBufferSize),
		byProject: make(map[string]int),
		byVersion: make(map[string]int),
		byTaskID:  make(map[string]int),
	}
}

func (s *statsCache) resetCache() {
	s.calls = 0
	s.total = 0
	s.byProject = make(map[string]int)
	s.byVersion = make(map[string]int)
	s.byTaskID = make(map[string]int)
}

func (s *statsCache) cacheStat(newStat Stat) {
	s.calls++
	s.total += newStat.Count
	s.byProject[newStat.Project] += newStat.Count
	s.byVersion[newStat.Version] += newStat.Count
	s.byTaskID[newStat.TaskID] += newStat.Count
}

func (s *statsCache) startConsumerLoop(ctx context.Context) {
	defer func() {
		if err := recovery.HandlePanicWithError(recover(), nil, "stats cache consumer"); err != nil {
			grip.Error(message.WrapError(err, message.Fields{
				"message": "panic in stats cache consumer loop",
				"cache":   s.cacheName,
			}))
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case nextStat := <-s.statChan:
			s.cacheStat(nextStat)
		}
	}
}

func (s *statsCache) startLoggerLoop(ctx context.Context) {
	defer func() {
		if err := recovery.HandlePanicWithError(recover(), nil, "stats cache logger"); err != nil {
			grip.Error(message.WrapError(err, message.Fields{
				"message": "panic in stats cache logger loop",
				"cache":   s.cacheName,
			}))
		}
	}()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.logStats()
		}
	}
}

func (s *statsCache) logStats() {
	grip.Info(message.Fields{
		"message":    fmt.Sprintf("%s stats", s.cacheName),
		"calls":      s.calls,
		"total":      s.total,
		"by_project": topNMap(s.byProject, topN),
		"by_version": topNMap(s.byVersion, topN),
		"by_task_id": topNMap(s.byTaskID, topN),
	})

	s.resetCache()
}

// AddStat adds a stat to the cache's incoming stats channel.
// Returns an error when the channel is full.
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
