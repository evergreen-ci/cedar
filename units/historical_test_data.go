package units

import (
	"context"
	"strings"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/pkg/errors"
)

func init() {
	registry.AddJobType(historicalTestDataJobName, func() amboy.Job { return makeHistoricalTestDataJob() })
}

const historicalTestDataJobName = "historical-test-data"

type historicalTestDataJob struct {
	job.Base `bson:"job_base" json:"job_base" yaml:"job_base"`
	Info     model.HistoricalTestDataInfo `bson:"info" json:"info" yaml:"info"`
	Result   model.TestResult             `bson:"result" json:"result" yaml:"result"`

	env cedar.Environment
}

// NewHistoricalTestDataJob returns a job that re-computes and stores the
// historical test data based on the new incoming test result.
func NewHistoricalTestDataJob(env cedar.Environment, info model.TestResultsInfo, tr model.TestResult) amboy.Job {
	j := makeHistoricalTestDataJob()

	j.env = env
	j.Result = tr
	taskName := j.getTaskName(info)
	j.Info = model.HistoricalTestDataInfo{
		Project:     info.Project,
		Variant:     info.Variant,
		TaskName:    taskName,
		TestName:    tr.TestName,
		RequestType: info.RequestType,
		Date:        tr.TestEndTime.UTC(),
	}
	j.SetID(strings.Join([]string{historicalTestDataJobName, j.Info.Project, j.Info.Variant, j.Info.TaskName, j.Info.TestName, j.Info.RequestType, j.Info.Date.String()}, "."))

	return j
}

func makeHistoricalTestDataJob() *historicalTestDataJob {
	j := &historicalTestDataJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    historicalTestDataJobName,
				Version: 0,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

// getTaskName returns the task name to attribute to the. The display task name
// is always prioritized over the execution task name, if the task is part of a
// display task.
func (j *historicalTestDataJob) getTaskName(info model.TestResultsInfo) string {
	if info.DisplayTaskName != "" {
		return info.DisplayTaskName
	}
	return info.TaskName
}

func (j *historicalTestDataJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}

	conf := model.NewCedarConfig(j.env)
	if err := conf.Find(); err != nil {
		j.AddError(errors.Wrap(err, "finding cedar configuration"))
		return
	}
	if conf.Flags.DisableHistoricalTestData {
		return
	}

	htd, err := model.CreateHistoricalTestData(j.Info)
	if err != nil {
		j.AddError(errors.Wrap(err, "creating historical test data"))
		return
	}
	htd.Setup(j.env)
	j.AddError(errors.Wrap(htd.Update(ctx, j.Result), "updating historical test data"))
}
