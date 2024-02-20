package units

import (
	"context"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

const testDBName = "cedar_test_units"

func init() {
	env, err := cedar.NewEnvironment(context.Background(), testDBName, &cedar.Configuration{
		MongoDBURI:                "mongodb://localhost:27017",
		DatabaseName:              testDBName,
		SocketTimeout:             time.Minute,
		NumWorkers:                2,
		DbConfigurationCollection: "configuration",
	})
	if err != nil {
		panic(err)
	}

	cedar.SetEnvironment(env)
}

func tearDownEnv(env cedar.Environment) error {
	conf, session, err := cedar.GetSessionWithConfig(env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()
	return errors.WithStack(session.DB(conf.DatabaseName).DropDatabase())
}

func TestAllRegisteredUnitsAreRemoteSafe(t *testing.T) {
	assert := assert.New(t)

	for id := range registry.JobTypeNames() {
		if id == "bond-recall-download-file" {
			continue
		}

		grip.Infoln("testing job is remote ready:", id)
		factory, err := registry.GetJobFactory(id)
		assert.NoError(err)
		assert.NotNil(factory)
		job := factory()

		assert.NotNil(job)

		assert.Equal(id, job.Type().Name)

		for _, f := range []amboy.Format{amboy.JSON, amboy.JSON} {
			assert.NotPanics(func() {
				dbjob, err := registry.MakeJobInterchange(job, f)

				assert.NoError(err)
				assert.NotNil(dbjob)
				assert.NotNil(dbjob.Dependency)
				assert.Equal(id, dbjob.Type)
			}, id)
		}
	}
}
