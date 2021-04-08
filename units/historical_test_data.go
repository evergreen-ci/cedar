package units

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/evergreen"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/pkg/errors"
)

const historicalTestDataJobName = "historical-test-data"

type historicalTestDataJob struct {
	job.Base `bson:"job_base" json:"job_base" yaml:"job_base"`
	Info     model.HistoricalTestDataInfo `bson:"info" json:"info" yaml:"info"`
	Result   model.TestResult             `bson:"result" json:"result" yaml:"result"`

	env      cedar.Environment
	settings historicalTestDataProjectSettings
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

	skip, err := j.shouldNoop(ctx, conf)
	j.AddError(errors.Wrapf(err, "checking if job needs to run"))
	if skip {
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

// shouldNoop checks if this job must run for this project and task, depending
// on whether historical test data is disabled for this job's project and
// whether this project skips tasks matching this job's task name.
func (j *historicalTestDataJob) shouldNoop(ctx context.Context, conf *model.CedarConfig) (bool, error) {
	settings, err := j.getProjectSettings(ctx, conf)
	if err != nil {
		return false, errors.Wrapf(err, "getting historical test data settings for project '%s'", j.Info.Project)
	}
	if settings.DisabledTestDataCache {
		return true, nil
	}

	skip, err := j.shouldSkipTask(settings)
	if skip || err != nil {
		return skip, errors.Wrapf(err, "checking if task '%s' should be skipped", j.Info.TaskName)
	}

	return false, nil
}

// shouldSkipTask checks whether this job's project skips tasks matching this
// job's task name.
func (j *historicalTestDataJob) shouldSkipTask(settings *historicalTestDataProjectSettings) (skip bool, err error) {
	for _, pattern := range j.settings.FilesIgnoredFromCache {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false, errors.Wrapf(err, "compiling regexp '%s'", pattern)
		}
		if re.MatchString(j.Info.TaskName) {
			return true, nil
		}
	}
	return false, nil
}

// historicalTestDataProjectSettings represents a subset of an Evergreen
// project's settings related to caching historical test data.
type historicalTestDataProjectSettings struct {
	DisabledTestDataCache bool     `json:"disabled_stats_cache"`
	FilesIgnoredFromCache []string `json:"files_ignored_from_cache"`
}

// getProjectSettings retrieves the cached historical test data project settings
// from Evergreen.
func (j *historicalTestDataJob) getProjectSettings(ctx context.Context, conf *model.CedarConfig) (*historicalTestDataProjectSettings, error) {
	client := utility.GetHTTPClient()
	defer utility.PutHTTPClient(client)
	evgClient := evergreen.NewClient(client, evergreen.ConnectionOptions{
		RootURL: conf.Evergreen.URL,
		User:    conf.Evergreen.ServiceUserName,
		Key:     conf.Evergreen.ServiceUserAPIKey,
	})

	resp, _, err := evgClient.Get(ctx, fmt.Sprintf("/projects/%s", j.Info.Project))
	if err != nil {
		return nil, errors.Wrap(err, "requesting project settings")
	}
	settings := &historicalTestDataProjectSettings{}
	if err := json.Unmarshal(resp, settings); err != nil {
		return nil, errors.Wrapf(err, "reading historical test data settings from project '%s'", j.Info.Project)
	}
	return settings, nil
}
