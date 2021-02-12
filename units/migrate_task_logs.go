package units

import (
	"context"
	"fmt"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/pkg/errors"
)

// TODO: Remove this job once task log migration is complete (EVG-13831).
// This is a temporary migration job. We need to take all task logs (logs with
// "info.proc_name" == "agent_log", "task_log", or "system_log") in the
// database and push the process name into the "info.tags" array. There are
// an estimated ~180,000,000 documents that need to be migrated so we will do
// this in smallish bathches over a few weeks to avoid overwhelming the prod
// database.

const (
	migrateTaskLogsJobName = "migrate-task-logs"
)

type migrateTaskLogsJob struct {
	Limit    int64 `bson:"limit" json:"limit" yaml:"limit"`
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`

	env cedar.Environment
}

func init() {
	registry.AddJobType(migrateTaskLogsJobName, func() amboy.Job { return makeMigrateTaskLogsJob() })
}

func makeMigrateTaskLogsJob() *migrateTaskLogsJob {
	j := &migrateTaskLogsJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    migrateTaskLogsJobName,
				Version: 1,
			},
		},
	}

	j.SetDependency(dependency.NewAlways())
	return j
}

func NewMigrateTaskLogsJob(limit int64) (amboy.Job, error) {
	if limit <= 0 {
		return nil, errors.New("task log migration limit must be greater than 0")
	}

	j := makeMigrateTaskLogsJob()

	ts := utility.RoundPartOfMinute(0).Format(tsFormat)
	j.SetID(fmt.Sprintf("%s.%s", migrateTaskLogsJobName, ts))
	j.SetScopes([]string{migrateTaskLogsJobName})
	j.Limit = limit

	return j, nil
}

func (j *migrateTaskLogsJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}

	j.AddError(errors.Wrap(model.FindAndUpdateOutdatedTaskLogs(ctx, j.env, j.Limit),
		"problem finding and updating outdated task logs"))
}
