package cedar

// config_auth_test.go contains tests for actually connecting to a db with auth enabled,
// assuming we already have read in the credentials from a file or other source

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// adapted from environment_test

func TestGlobalEnvironmentAuth(t *testing.T) {
	assert.Exactly(t, globalEnv, GetEnvironment())

	first := GetEnvironment()
	first.(*envState).name = "cedar-init"
	assert.Exactly(t, globalEnv, GetEnvironment())

	conf := &Configuration{
		MongoDBURI:   "mongodb://localhost:27017",
		NumWorkers:   2,
		DatabaseName: testDatabaseName,
		DBPwd:        "default",
	}
	uname := os.Getenv("TEST_USER_ADMIN")
	conf.DBUser = uname

	env, err := NewEnvironment(context.TODO(), "second", conf)
	assert.NoError(t, err)
	SetEnvironment(env)
	second := GetEnvironment()
	assert.NotEqual(t, first, second)
}

func TestDatabaseSessionAccessorAuth(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var env Environment

	_, _, err := GetSessionWithConfig(env)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is nil")
	conf1 := &Configuration{MongoDBURI: "mongodb://localhost:27017", DBPwd: "default"}
	uname := os.Getenv("TEST_USER_ADMIN")
	conf1.DBUser = uname
	env, err = NewEnvironment(ctx, "test", conf1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "amboy workers")
	_, _, err = GetSessionWithConfig(env)
	assert.Error(t, err)

	conf1 = &Configuration{MongoDBURI: "mongodb://localhost:27017", DBPwd: "default"}
	conf1.DBUser = uname
	conf1.NumWorkers = 2
	conf1.DatabaseName = testDatabaseName
	env, err = NewEnvironment(ctx, "test", conf1)
	assert.NoError(t, err)
	env.(*envState).client = nil
	_, _, err = GetSessionWithConfig(env)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session is nil")

	env, err = NewEnvironment(ctx, "test", conf1)
	assert.NoError(t, err)
	conf, db, err := GetSessionWithConfig(env)
	assert.NoError(t, err)
	assert.NotNil(t, conf)
	assert.NotNil(t, db)
}

// adopted from env test
func TestEnvironmentConfigurationAuth(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const ename = "cedar-test-env"
	for name, test := range map[string]func(t *testing.T, conf *Configuration){
		"VerifyFixturesAuth": func(t *testing.T, conf *Configuration) {
			assert.NotNil(t, conf)
			assert.NoError(t, conf.Validate())
		},
		"HasAuth": func(t *testing.T, conf *Configuration) {
			uname := os.Getenv("TEST_USER_ADMIN")
			if uname == "" {
				assert.False(t, conf.HasAuth())
			} else {
				assert.True(t, conf.HasAuth())
			}
		},
		"ValidConfigUsesLocalConfigAuth": func(t *testing.T, conf *Configuration) {
			conf.DisableRemoteQueue = true
			conf.DisableRemoteQueueGroup = true

			env, err := NewEnvironment(ctx, ename, conf)
			require.NoError(t, err)
			q := env.GetLocalQueue()
			require.NotNil(t, q)
			assert.False(t, strings.Contains(fmt.Sprintf("%T", q), "remote"))
		},
		// "": func(t *testing.T, conf *Configuration) {},
	} {
		t.Run(name, func(t *testing.T) {
			conf := &Configuration{
				MongoDBURI:         "mongodb://localhost:27017",
				NumWorkers:         2,
				MongoDBDialTimeout: time.Second,
				SocketTimeout:      10 * time.Second,
				DatabaseName:       testDatabaseName,
				DBPwd:              "default",
			}
			// setting DBUser to "" if we're not testing auth
			uname := os.Getenv("TEST_USER_ADMIN")
			conf.DBUser = uname
			test(t, conf)
		})
	}
}
