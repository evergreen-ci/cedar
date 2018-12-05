package cedar

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGlobalEnvironment(t *testing.T) {
	assert.Exactly(t, globalEnv, GetEnvironment())

	first := GetEnvironment()
	first.(*envState).name = "foo"
	assert.Exactly(t, globalEnv, GetEnvironment())

	resetEnv()
	second := GetEnvironment()
	assert.NotEqual(t, first, second)
}

func TestDatabaseSessionAccessor(t *testing.T) {
	env := &envState{}
	_, _, err := GetSessionWithConfig(env)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "configuration is not set")

	// this doesn't error because we don't require a db to be
	// setup
	err = env.Configure(&Configuration{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must specify")

	assert.Nil(t, env.session)
	_, _, err = GetSessionWithConfig(env)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "problem getting configuration")

	err = env.Configure(&Configuration{MongoDBURI: "mongodb://localhost:27017"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "amboy workers")
	_, _, err = GetSessionWithConfig(env)
	assert.Error(t, err)

	err = env.Configure(&Configuration{MongoDBURI: "mongodb://localhost:27017", NumWorkers: 2})
	assert.NoError(t, err)

	env.session = nil
	_, _, err = GetSessionWithConfig(env)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db session")

	assert.NoError(t, env.Configure(&Configuration{MongoDBURI: "mongodb://localhost:27017", NumWorkers: 2}))
	conf, db, err := GetSessionWithConfig(env)
	assert.NoError(t, err)
	assert.NotNil(t, conf)
	assert.NotNil(t, db)
}

func TestEnvironmentConfiguration(t *testing.T) {
	for name, test := range map[string]func(t *testing.T, env Environment, conf *Configuration){
		"ErrorsForInvalidConfig": func(t *testing.T, env Environment, conf *Configuration) {
			assert.Error(t, env.Configure(&Configuration{}))
		},
		"PanicsWithNilConfig": func(t *testing.T, env Environment, conf *Configuration) {
			assert.Panics(t, func() {
				_ = env.Configure(nil)
			})
		},
		"ErrorsWithMongoDBThatDoesNotExist": func(t *testing.T, env Environment, conf *Configuration) {
			conf.MongoDBURI = " NOT A SERVER "

			assert.Error(t, env.Configure(conf))
		},
		"VerifyFixtures": func(t *testing.T, env Environment, conf *Configuration) {
			assert.NotNil(t, env)
			assert.NotNil(t, conf)
			assert.NoError(t, conf.Validate())
		},
		"ValidConfigUsesLocalConfig": func(t *testing.T, env Environment, conf *Configuration) {
			conf.UseLocalQueue = true
			assert.NoError(t, env.Configure(conf))
			q, err := env.GetQueue()
			assert.NoError(t, err)
			assert.NotNil(t, q)
			assert.False(t, strings.Contains(fmt.Sprintf("%T", q), "remote"))
		},
		"DefaultsToRemoteQueueType": func(t *testing.T, env Environment, conf *Configuration) {
			assert.NoError(t, env.Configure(conf))
			q, err := env.GetQueue()
			assert.NoError(t, err)
			assert.NotNil(t, q)
			assert.True(t, strings.Contains(fmt.Sprintf("%T", q), "remote"))
		},
		// "": func(t *testing.T, env Environment, conf *Configuration) {},
	} {
		t.Run(name, func(t *testing.T) {
			env := &envState{}
			conf := &Configuration{
				MongoDBURI:         "mongodb://localhost:27017",
				NumWorkers:         2,
				MongoDBDialTimeout: 10 * time.Millisecond,
			}
			test(t, env, conf)
		})
	}
}
