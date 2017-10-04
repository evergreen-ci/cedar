package units

import (
	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/cost"
	"github.com/evergreen-ci/sink/model"
	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
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
	env      sink.Environment
}

func NewBuildCostReport(env sink.Environment, name string, opts *cost.EvergreenReportOptions) amboy.Job {
	j := makeBuildCostReport()

	j.env = env
	j.Options = *opts
	j.SetID(name)
	return j
}

func (j *buildCostReportJob) Run() {
	defer j.MarkComplete()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	grip.Infoln("running build cost reporting job:", j.ID())

	if j.env == nil {
		j.env = sink.GetEnvironment()
	}

	costConf := &model.CostConfig{}
	costConf.Setup(j.env)
	if err := costConf.Find(); err != nil {
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
	output.Setup(j.env)
	output.ID = j.ID()
	if err := output.Save(); err != nil {
		grip.Warning(err)
		j.AddError(err)
		return
	}

	summary := model.NewCostReportSummary(output)
	summary.Setup(j.env)
	if err := summary.Save(); err != nil {
		grip.Warning(err)
		j.AddError(err)
		return
	}

	grip.Notice(message.Fields{
		"id":      "build-cost-report",
		"state":   "output",
		"period":  j.Options.Duration.String(),
		"starts":  j.Options.StartAt,
		"summary": summary,
	})
}
