package cedar

import (
	"context"
	"testing"
	"time"

	"github.com/evergreen-ci/utility"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvironmentCache(t *testing.T) {
	t.Run("PutNewGetAndDelete", func(t *testing.T) {
		cache := newEnvironmentCache()

		require.True(t, cache.PutNew("key0", "val0"))
		assert.False(t, cache.PutNew("key0", "val0"))
		require.True(t, cache.PutNew("key1", 1))
		assert.False(t, cache.PutNew("key1", 1))

		val0, ok := cache.Get("key0")
		assert.True(t, ok)
		assert.Equal(t, "val0", val0)

		val1, ok := cache.Get("key1")
		assert.True(t, ok)
		assert.Equal(t, 1, val1)

		cache.Delete("key0")
		val0, ok = cache.Get("key0")
		assert.False(t, ok)
		assert.Nil(t, val0)
	})
	t.Run("WithUpdater", func(t *testing.T) {
		cache := newEnvironmentCache()

		key0 := "key0"
		val0 := "value0"
		updates0 := make(chan interface{})
		ctx0, cancel0 := context.WithCancel(context.Background())
		require.True(t, cache.PutNew(key0, val0))
		require.True(t, cache.RegisterUpdater(ctx0, cancel0, key0, updates0))

		key1 := "key1"
		val1 := 5
		updates1 := make(chan interface{})
		ctx1, cancel1 := context.WithCancel(context.Background())
		require.True(t, cache.PutNew(key1, val1))
		require.True(t, cache.RegisterUpdater(ctx1, cancel1, key1, updates1))

		t.Run("ReturnsInitialValue", func(t *testing.T) {
			val, ok := cache.Get(key0)
			require.True(t, ok)
			assert.Equal(t, val0, val)

			val, ok = cache.Get(key1)
			require.True(t, ok)
			assert.Equal(t, val1, val)
		})
		t.Run("FailsWhenUpdaterAlreadyExists", func(t *testing.T) {
			assert.False(t, cache.RegisterUpdater(ctx0, cancel0, key0, make(chan interface{})))
		})
		t.Run("FailsWhenKeyDNE", func(t *testing.T) {
			assert.False(t, cache.RegisterUpdater(ctx0, cancel0, "DNE", make(chan interface{})))
		})
		t.Run("ReturnsUpdatedValue", func(t *testing.T) {
			var val interface{}

			lastVal, ok := cache.Get(key0)
			require.True(t, ok)
			newVal0 := "new_value0"
			updates0 <- newVal0
			retryOp := func() (bool, error) {
				val, ok = cache.Get(key0)
				if !ok {
					return false, errors.New("value not found in cache")
				}
				if lastVal == val {
					return true, errors.New("value not updated")
				}
				return false, nil
			}
			assert.NoError(t, utility.Retry(context.TODO(), retryOp, utility.RetryOptions{MaxAttempts: 5}))
			assert.Equal(t, newVal0, val)

			lastVal, ok = cache.Get(key1)
			require.True(t, ok)
			newVal1 := 10
			updates1 <- newVal1
			retryOp = func() (bool, error) {
				val, ok = cache.Get(key1)
				if !ok {
					return false, errors.New("value not found in cache")
				}
				if lastVal == val {
					return true, errors.New("value not updated")
				}
				return false, nil
			}
			assert.NoError(t, utility.Retry(context.TODO(), retryOp, utility.RetryOptions{MaxAttempts: 5}))
			assert.Equal(t, newVal1, val)
		})
		t.Run("DeletesValueOnUpdateErr", func(t *testing.T) {
			err := errors.New("some error")
			updates0 <- err

			timer := time.NewTimer(time.Second)
			defer timer.Stop()
			select {
			case <-ctx0.Done():
			case <-timer.C:
			}
			assert.Error(t, ctx0.Err())
			_, ok := cache.Get(key0)
			assert.False(t, ok)
			_, ok = cache.updaterClosers[key0]
			assert.False(t, ok)

			lastVal, ok := cache.Get(key1)
			newVal1 := 20
			updates1 <- newVal1
			var val interface{}
			retryOp := func() (bool, error) {
				val, ok = cache.Get(key1)
				if !ok {
					return false, errors.New("value not found in cache")
				}
				if lastVal == val {
					return true, errors.New("value not updated")
				}
				return false, nil
			}
			assert.NoError(t, utility.Retry(context.TODO(), retryOp, utility.RetryOptions{MaxAttempts: 5}))
			assert.Equal(t, newVal1, val)
		})
		t.Run("DeletesValueOnCtxErr", func(t *testing.T) {
			cancel1()
			retryOp := func() (bool, error) {
				_, ok := cache.Get(key1)
				if ok {
					return true, errors.New("value still found in cache")
				}
				return false, nil
			}
			assert.NoError(t, utility.Retry(context.TODO(), retryOp, utility.RetryOptions{MaxAttempts: 5}))
			_, ok := cache.updaterClosers[key1]
			assert.False(t, ok)
		})
	})
}
