package cedar

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatsCache(t *testing.T) {
	t.Run("cache is full", func(t *testing.T) {
		cache := newStatsCache("new cache")
		for i := 0; i < statChanBufferSize; i++ {
			assert.NoError(t, cache.AddStat(Stat{}))
		}
		assert.Error(t, cache.AddStat(Stat{}))
	})
	t.Run("logStats clears the cache", func(t *testing.T) {
		cache := newStatsCache("new cache")
		cache.calls++
		cache.logStats()
		assert.Equal(t, cache.calls, 0)
	})
	t.Run("stat is recorded", func(t *testing.T) {
		cache := newStatsCache("new cache")
		cache.cacheStat(Stat{
			Count:   2,
			Project: "cedar",
			Version: "abcdef",
			TaskID:  "t1",
		})
		assert.Equal(t, cache.calls, 1)
		assert.Equal(t, cache.total, 2)
		assert.Equal(t, cache.byProject["cedar"], 2)
		assert.Equal(t, cache.byVersion["abcdef"], 2)
		assert.Equal(t, cache.byTaskID["t1"], 2)
	})
	t.Run("topNItems", func(t *testing.T) {
		t.Run("empty map", func(t *testing.T) {
			assert.Empty(t, topNItems(map[string]int{}, 10))
		})
		t.Run("nil map", func(t *testing.T) {
			assert.Empty(t, topNItems(nil, 10))
		})
		t.Run("fewer than n", func(t *testing.T) {
			fullMap := map[string]int{
				"one": 1,
			}
			items := topNItems(fullMap, 10)
			require.Len(t, items, 1)
			assert.Equal(t, items[0].Identifier, "one")
		})
		t.Run("greater than n", func(t *testing.T) {
			fullMap := map[string]int{
				"one":   1,
				"two":   2,
				"three": 3,
			}
			items := topNItems(fullMap, 2)
			require.Len(t, items, 2)
			assert.Equal(t, items[0].Identifier, "three")
			assert.Equal(t, items[1].Identifier, "two")
		})
	})
}
