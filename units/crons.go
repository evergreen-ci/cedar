package units

import (
	"context"
	"fmt"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/amboy"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

const tsFormat = "2006-01-02.15-04-05"

func StartCrons(ctx context.Context, env cedar.Environment, rpcTLS bool) error {
	opts := amboy.QueueOperationConfig{
		ContinueOnError: true,
		LogErrors:       false,
		DebugLogging:    false,
	}

	remote := env.GetRemoteQueue()
	local := env.GetLocalQueue()

	grip.Info(message.Fields{
		"message": "starting background cron jobs",
		"state":   "not populated",
		"opts":    opts,
		"started": message.Fields{
			"remote": remote.Info().Started,
			"local":  local.Info().Started,
		},
		"stats": message.Fields{
			"remote": remote.Stats(ctx),
			"local":  local.Stats(ctx),
		},
	})

	amboy.IntervalQueueOperation(ctx, local, time.Minute, time.Now(), opts, func(ctx context.Context, queue amboy.Queue) error {
		conf := model.NewCedarConfig(env)
		if err := conf.Find(); err != nil {
			return errors.WithStack(err)
		}

		if conf.Flags.DisableInternalMetricsReporting {
			return nil
		}

		ts := utility.RoundPartOfMinute(0).Format(tsFormat)
		catcher := grip.NewBasicCatcher()
		catcher.Add(queue.Put(ctx, NewSysInfoStatsCollector(fmt.Sprintf("sys-info-stats-%s", ts))))
		catcher.Add(queue.Put(ctx, NewLocalAmboyStatsCollector(env, ts)))
		catcher.Add(queue.Put(ctx, NewJasperManagerCleanup(ts, env)))
		return catcher.Resolve()
	})
	amboy.IntervalQueueOperation(ctx, remote, time.Minute, time.Now(), opts, func(ctx context.Context, queue amboy.Queue) error {
		conf := model.NewCedarConfig(env)
		if err := conf.Find(); err != nil {
			return errors.WithStack(err)
		}

		if conf.Flags.DisableInternalMetricsReporting {
			return nil
		}

		return queue.Put(ctx, NewRemoteAmboyStatsCollector(env, utility.RoundPartOfMinute(0).Format(tsFormat)))
	})
	/*
		This is disabled for https://jira.mongodb.org/browse/EVG-19321. The Perf Build Baron Team would currently prefer
		that historical data not change in order to not impact older build failures.

		amboy.IntervalQueueOperation(ctx, remote, time.Hour, time.Now(), opts, func(ctx context.Context, queue amboy.Queue) error {
			job, err := NewFindOutdatedRollupsJob(perf.DefaultRollupFactories())
			if err != nil {
				return errors.WithStack(err)
			}

			return queue.Put(ctx, job)
		})
	*/
	amboy.IntervalQueueOperation(ctx, remote, 10*time.Minute, time.Now(), opts, func(ctx context.Context, queue amboy.Queue) error {
		conf := model.NewCedarConfig(env)
		if err := conf.Find(); err != nil {
			return errors.WithStack(err)
		}
		if conf.Flags.DisableSignalProcessing {
			return nil
		}
		job := NewPeriodicTimeSeriesUpdateJob(utility.RoundPartOfHour(10).Format(tsFormat))
		return queue.Put(ctx, job)
	})
	amboy.IntervalQueueOperation(ctx, remote, 24*time.Hour, time.Now(), opts, func(ctx context.Context, queue amboy.Queue) error {
		conf := model.NewCedarConfig(env)
		if err := conf.Find(); err != nil {
			return errors.WithStack(err)
		}

		if conf.Flags.DisableInternalMetricsReporting {
			return nil
		}

		return queue.Put(ctx, NewStatsDBCollectionSizeJob(env, utility.RoundPartOfMinute(0).Format(tsFormat)))
	})

	return nil
}
