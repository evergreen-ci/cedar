package cedar

// config_auth_test.go contains tests for actually connecting to a db with auth enabled,
// assuming we already have read in the credentials from a file or other source

import (
	"context"
	"fmt"
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
		DBUser:       "myUserAdmin",
		DBPwd:        "default",
	}

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

	env, err = NewEnvironment(ctx, "test", &Configuration{MongoDBURI: "mongodb://localhost:27017", DBUser: "myUserAdmin", DBPwd: "default"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "amboy workers")
	_, _, err = GetSessionWithConfig(env)
	assert.Error(t, err)

	env, err = NewEnvironment(ctx, "test", &Configuration{MongoDBURI: "mongodb://localhost:27017", NumWorkers: 2, DatabaseName: testDatabaseName, DBUser: "myUserAdmin", DBPwd: "default"})
	assert.NoError(t, err)
	env.(*envState).client = nil
	_, _, err = GetSessionWithConfig(env)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session is nil")

	env, err = NewEnvironment(ctx, "test", &Configuration{MongoDBURI: "mongodb://localhost:27017", NumWorkers: 2, DatabaseName: testDatabaseName, DBUser: "myUserAdmin", DBPwd: "default"})
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
			assert.True(t, conf.HasAuth())
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
				DBUser:             "myUserAdmin",
				DBPwd:              "default",
			}
			test(t, conf)
		})
	}
}
