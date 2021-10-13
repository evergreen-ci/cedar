package cedar

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/evergreen-ci/utility"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testDatabaseName = "cedar_test"

func TestGlobalEnvironment(t *testing.T) {
	assert.Exactly(t, globalEnv, GetEnvironment())

	first := GetEnvironment()
	first.(*envState).name = "cedar-init"
	assert.Exactly(t, globalEnv, GetEnvironment())

	env, err := NewEnvironment(context.TODO(), "second", &Configuration{MongoDBURI: "mongodb://localhost:27017", NumWorkers: 2, DatabaseName: testDatabaseName})
	assert.NoError(t, err)
	SetEnvironment(env)
	second := GetEnvironment()
	assert.NotEqual(t, first, second)
}

func TestDatabaseSessionAccessor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var env Environment

	_, _, err := GetSessionWithConfig(env)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is nil")

	env, err = NewEnvironment(ctx, "test", &Configuration{MongoDBURI: "mongodb://localhost:27017"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "amboy workers")
	_, _, err = GetSessionWithConfig(env)
	assert.Error(t, err)

	env, err = NewEnvironment(ctx, "test", &Configuration{MongoDBURI: "mongodb://localhost:27017", NumWorkers: 2, DatabaseName: testDatabaseName})
	assert.NoError(t, err)
	env.(*envState).client = nil
	_, _, err = GetSessionWithConfig(env)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session is nil")

	env, err = NewEnvironment(ctx, "test", &Configuration{MongoDBURI: "mongodb://localhost:27017", NumWorkers: 2, DatabaseName: testDatabaseName})
	assert.NoError(t, err)
	conf, db, err := GetSessionWithConfig(env)
	assert.NoError(t, err)
	assert.NotNil(t, conf)
	assert.NotNil(t, db)
}

func TestEnvironmentConfiguration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const ename = "cedar-test-env"
	for name, test := range map[string]func(t *testing.T, conf *Configuration){
		"ErrorsForInvalidConfig": func(t *testing.T, conf *Configuration) {
			env, err := NewEnvironment(ctx, ename, &Configuration{})
			assert.Error(t, err)
			assert.Nil(t, env)
		},
		"PanicsWithNilConfig": func(t *testing.T, conf *Configuration) {
			assert.Panics(t, func() {
				_, _ = NewEnvironment(ctx, ename, nil)
			})
		},
		"ErrorsWithMongoDBThatDoesNotExist": func(t *testing.T, conf *Configuration) {
			conf.MongoDBURI = "mongodb://localhost:27016"

			env, err := NewEnvironment(ctx, ename, conf)
			assert.Error(t, err)
			assert.Nil(t, env)
		},
		"VerifyFixtures": func(t *testing.T, conf *Configuration) {
			assert.NotNil(t, conf)
			assert.NoError(t, conf.Validate())
		},
		"ValidConfigUsesLocalConfig": func(t *testing.T, conf *Configuration) {
			conf.DisableRemoteQueue = true
			conf.DisableRemoteQueueGroup = true

			env, err := NewEnvironment(ctx, ename, conf)
			require.NoError(t, err)
			q := env.GetLocalQueue()
			require.NotNil(t, q)
			assert.False(t, strings.Contains(fmt.Sprintf("%T", q), "remote"))
		},
		"DefaultsToRemoteQueueType": func(t *testing.T, conf *Configuration) {
			if runtime.GOOS == "windows" {
				t.Skip("windows support is not required")
			}
			env, err := NewEnvironment(ctx, ename, conf)
			require.NoError(t, err)
			q := env.GetRemoteQueue()
			require.NotNil(t, q)
			assert.True(t, strings.Contains(fmt.Sprintf("%T", q), "remote"))
		},
	} {
		t.Run(name, func(t *testing.T) {
			conf := &Configuration{
				MongoDBURI:         "mongodb://localhost:27017",
				NumWorkers:         2,
				MongoDBDialTimeout: time.Second,
				SocketTimeout:      10 * time.Second,
				DatabaseName:       testDatabaseName,
			}
			test(t, conf)
		})
	}
}

func TestEnvironmentDBValueCache(t *testing.T) {
	t.Run("DisabledDBValueCache", func(t *testing.T) {
		env, err := NewEnvironment(context.TODO(), "test", &Configuration{
			MongoDBURI:            "mongodb://localhost:27017",
			NumWorkers:            2,
			DatabaseName:          testDatabaseName,
			DisableDBValueCaching: true,
		})
		require.NoError(t, err)

		assert.False(t, env.RegisterDBValueCacher("some_value", "value", nil))
	})
	t.Run("EnabledDBValueCache", func(t *testing.T) {
		env, err := NewEnvironment(context.TODO(), "test", &Configuration{
			MongoDBURI:   "mongodb://localhost:27017",
			NumWorkers:   2,
			DatabaseName: testDatabaseName,
		})
		require.NoError(t, err)

		key0 := "key0"
		val0 := "value0"
		updateChan0 := make(chan interface{})
		require.True(t, env.RegisterDBValueCacher(key0, val0, updateChan0))
		key1 := "key1"
		val1 := 5
		updateChan1 := make(chan interface{})
		require.True(t, env.RegisterDBValueCacher(key1, val1, updateChan1))

		t.Run("ReturnsInitialValue", func(t *testing.T) {
			val, ok := env.GetCachedDBValue(key0)
			require.True(t, ok)
			assert.Equal(t, val0, val)

			val, ok = env.GetCachedDBValue(key1)
			require.True(t, ok)
			assert.Equal(t, val1, val)
		})
		t.Run("ReturnsUpdatedValue", func(t *testing.T) {
			var val interface{}

			lastVal, ok := env.GetCachedDBValue(key0)
			newVal0 := "new_value0"
			updateChan0 <- newVal0
			retyOp := func() (bool, error) {
				val, ok = env.GetCachedDBValue(key0)
				if !ok {
					return false, errors.New("value not found in cache")
				}
				if lastVal == val {
					return true, errors.New("value not updated")
				}
				return false, nil
			}
			assert.NoError(t, utility.Retry(context.TODO(), retyOp, utility.RetryOptions{MaxAttempts: 5}))
			assert.Equal(t, newVal0, val)

			lastVal, ok = env.GetCachedDBValue(key1)
			newVal1 := 10
			updateChan1 <- newVal1
			retyOp = func() (bool, error) {
				val, ok = env.GetCachedDBValue(key1)
				if !ok {
					return false, errors.New("value not found in cache")
				}
				if lastVal == val {
					return true, errors.New("value not updated")
				}
				return false, nil
			}
			assert.NoError(t, utility.Retry(context.TODO(), retyOp, utility.RetryOptions{MaxAttempts: 5}))
			assert.Equal(t, newVal1, val)
		})
		t.Run("DeletesValueOnChan", func(t *testing.T) {
			err := errors.New("some error")
			updateChan0 <- err
			time.Sleep(time.Millisecond)
			val, ok := env.GetCachedDBValue(key0)
			assert.False(t, ok)
			assert.Nil(t, val)

			_, ok = env.GetCachedDBValue(key1)
			assert.True(t, ok)
		})
	})
}
