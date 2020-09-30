package units

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/cost"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/pkg/errors"
)

const historicalStatsJobName = "historical-stats"

type historicalStatsJob struct {
	job.Base    `bson:"job_base" json:"job_base" yaml:"job_base"`
	ProjectID   string `bson:"project_id" json:"project_id" yaml:"project_id"`
	RequestType string `bson:"request_type" json:"request_type" yaml:"request_type"`

	env      cedar.Environment
	settings historicalStatsProjectSettings
}

func NewHistoricalStatsJob(env cedar.Environment, projectID, requestType string) amboy.Job {
	j := makeHistoricalStatsJob()
	j.env = env
	j.ProjectID = projectID
	j.RequestType = requestType
	return j
}
func makeHistoricalStatsJob() *historicalStatsJob {
	j := &historicalStatsJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    historicalStatsJobName,
				Version: 0,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func (j *historicalStatsJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}

	conf := model.NewCedarConfig(j.env)
	if err := conf.Find(); err != nil {
		j.AddError(errors.Wrap(err, "finding cedar configuration"))
		return
	}
	if conf.Flags.DisableHistoricalStats {
		return
	}

	if err := j.getHistoricalStatsSettings(ctx, conf); err != nil {
		j.AddError(errors.Wrapf(err, "getting historical stats settings for project '%s'", j.ProjectID))
		return
	}
	if j.settings.DisabledStatsCache {
		return
	}

	// TODO (EVG-8914): implement historical stats job
}

// getHistoricalStatsSettings gets the project settings from Evergreen.
func (j *historicalStatsJob) getHistoricalStatsSettings(ctx context.Context, conf *model.CedarConfig) error {
	client := utility.GetHTTPClient()
	defer utility.PutHTTPClient(client)
	connInfo := &model.EvergreenConnectionInfo{
		RootURL: conf.Evergreen.URL,
		User:    conf.Evergreen.ServiceUserName,
		Key:     conf.Evergreen.ServiceUserAPIKey,
	}
	evgClient := cost.NewEvergreenClient(client, connInfo)

	resp, _, err := evgClient.Get(ctx, fmt.Sprintf("/projects/%s", j.ProjectID))
	if err != nil {
		return errors.Wrap(err, "requesting project settings")
	}
	var settings historicalStatsProjectSettings
	if err := json.Unmarshal(resp, &settings); err != nil {
		return errors.Wrapf(err, "reading historical stats settings from project '%s'", j.ProjectID)
	}
	j.settings = settings
	return nil
}

type historicalStatsProjectSettings struct {
	DisabledStatsCache    bool     `json:"disabled_stats_cache"`
	FilesIgnoredFromCache []string `json:"files_ignored_from_cache"`
}
