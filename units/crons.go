package units

import (
	"context"
	"fmt"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/util"
	"github.com/mongodb/amboy"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

const tsFormat = "2006-01-02.15-04-05"

func StartCrons(ctx context.Context, env cedar.Environment) error {
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
			"remote": remote.Started(),
			"local":  local.Started(),
		},
		"stats": message.Fields{
			"remote": remote.Stats(),
			"local":  local.Stats(),
		},
	})

	amboy.IntervalQueueOperation(ctx, local, time.Minute, time.Now(), opts, func(queue amboy.Queue) error {
		conf := model.NewCedarConfig(env)
		if err := conf.Find(); err != nil {
			return errors.WithStack(err)
		}

		if conf.Flags.DisableInternalMetricsReporting {
			return nil
		}

		ts := util.RoundPartOfMinute(0).Format(tsFormat)
		catcher := grip.NewBasicCatcher()
		catcher.Add(queue.Put(NewSysInfoStatsCollector(fmt.Sprintf("sys-info-stats-%s", ts))))
		catcher.Add(queue.Put(NewLocalAmboyStatsCollector(env, ts)))
		return catcher.Resolve()
	})
	amboy.IntervalQueueOperation(ctx, remote, time.Minute, time.Now(), opts, func(queue amboy.Queue) error {
		conf := model.NewCedarConfig(env)
		if err := conf.Find(); err != nil {
			return errors.WithStack(err)
		}

		if conf.Flags.DisableInternalMetricsReporting {
			return nil
		}

		return queue.Put(NewRemoteAmboyStatsCollector(env, util.RoundPartOfMinute(0).Format(tsFormat)))
	})

	return nil
}
