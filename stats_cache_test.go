package cedar

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatsCache(t *testing.T) {
	t.Run("cache is full", func(t *testing.T) {
		cache := newStatsCache("new cache")
		for i := 0; i < statChanBufferSize; i++ {
			assert.NoError(t, cache.AddStat(Stat{}))
		}
		assert.Error(t, cache.AddStat(Stat{}))
	})
	t.Run("LogStats clears the cache", func(t *testing.T) {
		cache := newStatsCache("new cache")
		cache.calls++
		cache.LogStats()
		assert.Equal(t, cache.calls, 0)
	})
	t.Run("stat is recorded", func(t *testing.T) {
		cache := newStatsCache("new cache")
		cache.cacheStat(Stat{
			Count:   2,
			Project: "cedar",
			Version: "abcdef",
			Task:    "t1",
		})
		assert.Equal(t, cache.calls, 1)
		assert.Equal(t, cache.total, 2)
		assert.Equal(t, cache.byProject["cedar"], 2)
		assert.Equal(t, cache.byVersion["abcdef"], 2)
		assert.Equal(t, cache.byTask["t1"], 2)
	})
	t.Run("topNMap", func(t *testing.T) {
		t.Run("empty map", func(t *testing.T) {
			assert.Empty(t, topNMap(map[string]int{}, 10))
		})
		t.Run("nil map", func(t *testing.T) {
			assert.Empty(t, topNMap(nil, 10))
		})
		t.Run("fewer than n", func(t *testing.T) {
			fullMap := map[string]int{
				"one": 1,
			}
			newMap := topNMap(fullMap, 10)
			assert.Len(t, newMap, 2)
			assert.Contains(t, newMap, "one")
		})
		t.Run("greater than n", func(t *testing.T) {
			fullMap := map[string]int{
				"one":   1,
				"two":   2,
				"three": 3,
			}
			newMap := topNMap(fullMap, 2)
			assert.Len(t, newMap, 2)
			assert.Contains(t, newMap, "two")
			assert.Contains(t, newMap, "three")
		})
	})
}
