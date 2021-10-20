package cedar

import (
	"context"
	"fmt"
	"sync"

	"github.com/mongodb/grip/recovery"
)

// EnvironmentCache provides thread-safe, in-memory access to data persisted
// elsewhere, such as on disk or a database.
type EnvironmentCache interface {
	// PutNew adds new (key, value) pair to the cache.
	PutNew(string, interface{}) bool
	// RegisterUpdater registers a new cache value updater for the given
	// key. The updater should listen for updates on the given channel and
	// use the context and cancel function for signaling.
	RegisterUpdater(context.Context, context.CancelFunc, string, chan interface{}) bool
	// Get returns the value of the given key.
	Get(string) (interface{}, bool)
	// Delete removes the given key from the cache.
	Delete(string)
}

type envCache struct {
	mu             sync.RWMutex
	cache          map[string]interface{}
	updaterClosers map[string]context.CancelFunc
}

func newEnvironmentCache() *envCache {
	return &envCache{
		cache:          map[string]interface{}{},
		updaterClosers: map[string]context.CancelFunc{},
	}
}

// PutNew adds a new value to the cache with the given key name, returning true
// on success. If the key already exists, this noops and returns false.
func (c *envCache) PutNew(key string, value interface{}) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.cache[key]; !ok {
		c.cache[key] = value
		return true
	}

	return false
}

// RegisterUpdater creates a goroutine that listens on updates for the given
// key, returning true on success. The key must exist in the cache and it
// cannot already have an updater registered. Upon receiving an updated value,
// the updater will replace the current value of key with the new value. If an
// error is received, cancel is called and the goroutine exits. The lifespan of
// the updater is bound to ctx.
func (c *envCache) RegisterUpdater(ctx context.Context, cancel context.CancelFunc, key string, updates chan interface{}) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.cache[key]; !ok {
		return false
	}
	if _, ok := c.updaterClosers[key]; ok {
		return false
	}
	c.updaterClosers[key] = cancel

	go func() {
		defer func() {
			recovery.LogStackTraceAndContinue(fmt.Sprintf("env cache updater for '%s'", key))

			c.mu.Lock()
			delete(c.cache, key)
			delete(c.updaterClosers, key)
			cancel()
			c.mu.Unlock()
		}()

		for {
			select {
			case newValue := <-updates:
				switch newValue.(type) {
				case error:
					return
				default:
					c.mu.Lock()
					c.cache[key] = newValue
					c.mu.Unlock()
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return true
}

// Get returns the value of key and true if the key exists. Otherwise, nil and
// false are returned.
func (c *envCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	value, ok := c.cache[key]
	return value, ok
}

// Delete removes the given key from the cache and stops its updater, if
// applicable.
func (c *envCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.cache, key)
	if cancel, ok := c.updaterClosers[key]; ok {
		delete(c.updaterClosers, key)
		cancel()
	}
}
