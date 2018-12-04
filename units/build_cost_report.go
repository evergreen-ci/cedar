package units

import (
	"context"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/cost"
	"github.com/evergreen-ci/cedar/model"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

func init() {
	registry.AddJobType("build-cost-report", func() amboy.Job { return makeBuildCostReport() })
}

func makeBuildCostReport() *buildCostReportJob {
	j := &buildCostReportJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    "build-cost-report",
				Version: 2,
			},
		},
	}

	j.SetDependency(dependency.NewAlways())
	return j
}

type buildCostReportJob struct {
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
	Options  cost.EvergreenReportOptions `bson:"evg_opts" json:"evg_opts" yaml:"evg_opts"`
	env      cedar.Environment
}

func NewBuildCostReport(env cedar.Environment, name string, opts *cost.EvergreenReportOptions) amboy.Job {
	j := makeBuildCostReport()

	j.env = env
	j.Options = *opts
	j.SetID(name)
	return j
}

func (j *buildCostReportJob) Run(ctx context.Context) {
	defer grip.Infoln("completed job: ", j.ID())
	defer j.MarkComplete()
	grip.Infoln("running build cost reporting job:", j.ID())

	if j.env == nil {
		j.env = cedar.GetEnvironment()
	}

	conf := model.NewCedarConfig(j.env)
	if err := conf.Find(); err != nil {
		j.AddError(errors.WithStack(err))
		return
	}

	if conf.Flags.DisableCostReportingJob {
		return
	}

	costConf := &model.CostConfig{}
	costConf.Setup(j.env)
	if err := costConf.Find(); err != nil {
		grip.Warning(err)
		j.AddError(errors.WithStack(err))
		return
	}

	// run the report
	output, err := cost.CreateReport(ctx, costConf, &j.Options)
	if err != nil {
		grip.Warning(err)
		j.AddError(errors.WithStack(err))
		return
	}
	output.ID = j.ID()
	output.Setup(j.env)
	if err := output.Save(); err != nil {
		grip.Warning(err)
		j.AddError(err)
		return
	}

	summary := model.NewCostReportSummary(output)
	summary.Setup(j.env)
	if err := summary.Save(); err != nil {
		grip.Warning(err)
	}

	grip.Notice(message.Fields{
		"id":      "build-cost-report",
		"state":   "output",
		"period":  j.Options.Duration.String(),
		"starts":  j.Options.StartAt,
		"summary": summary,
	})
}
