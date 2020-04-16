package units

import (
	"context"
	"testing"
	"time"

	"github.com/mongodb/amboy/queue"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/evergreen-ci/cedar"
)

func init() {
	dbName := "test_cedar_signal_processing_periodic"
	env, err := cedar.NewEnvironment(context.Background(), dbName, &cedar.Configuration{
		MongoDBURI:    "mongodb://localhost:27017",
		DatabaseName:  dbName,
		SocketTimeout: time.Minute,
		NumWorkers:    2,
	})
	if err != nil {
		panic(err)
	}
	cedar.SetEnvironment(env)
}

func tearDownPeriodicTest(env cedar.Environment) error {
	conf, session, err := cedar.GetSessionWithConfig(env)
	if err != nil {
		return errors.WithStack(err)
	}
	defer session.Close()
	return errors.WithStack(session.DB(conf.DatabaseName).DropDatabase())
}

func TestPeriodicChangePointsJob(t *testing.T) {
	env := cedar.GetEnvironment()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = env.GetDB().Drop(ctx)

	aRollups, _ := makePerfResultsWithChangePoints("a", time.Now().UnixNano())
	bRollups, _ := makePerfResultsWithChangePoints("b", time.Now().UnixNano())
	provisionDb(ctx, env, append(aRollups, bRollups...))
	defer func() {
		assert.NoError(t, tearDownPeriodicTest(env))
	}()

	t.Run("PeriodicallySchedules", func(t *testing.T) {
		j := NewPeriodicChangePointJob("someId")
		job := j.(*periodicChangePointJob)
		job.queue = queue.NewLocalLimitedSize(1, 100)
		assert.NoError(t, job.queue.Start(ctx))
		j.Run(ctx)
		assert.True(t, j.Status().Completed)
		assert.Equal(t, job.queue.Stats(ctx).Total, 2)

	})
}
