package operations

import (
	"context"
	"testing"

	"github.com/evergreen-ci/cedar"
	"github.com/stretchr/testify/assert"
)

func TestServiceConfiguration(t *testing.T) {
	configure := func(env cedar.Environment, numWorkers int, localQueue bool, mongodbURI, bucket, dbName string) error {
		return newServiceConf(numWorkers, localQueue, mongodbURI, bucket, dbName).setup(context.TODO())
	}

	for name, test := range map[string]func(t *testing.T, env cedar.Environment){
		"VerifyFixtures": func(t *testing.T, env cedar.Environment) {
			assert.NotNil(t, env)
		},
		"ErrorsWithInvalidConfigDatabase": func(t *testing.T, env cedar.Environment) {
			err := configure(env, 2, true, "", "", "")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "mongodb")
		},
		"ErrorsWithInvalidConfigWorkers": func(t *testing.T, env cedar.Environment) {
			err := configure(env, -1, true, "mongodb://localhost:27017", "", "")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "workers")
		},
		"ValidOptions": func(t *testing.T, env cedar.Environment) {
			err := configure(env, 2, true, "mongodb://localhost:27017", "foo", "cedar_test")
			assert.NoError(t, err)
		},
		"ConfigurationOfLogging": func(t *testing.T, env cedar.Environment) {
			t.Skip("skipping because the code is improperly factored to support testing")
		},
		// "": func(t *testing.T, env cedar.Environment) {},
	} {
		t.Run(name, func(t *testing.T) {
			env := cedar.GetEnvironment()
			test(t, env)
		})
	}
}
