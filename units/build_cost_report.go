package units

import (
	"time"

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
				Version: 1,
			},
		},
	}

	j.SetDependency(dependency.NewAlways())
	return j
}

type buildCostReportJob struct {
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
	StartAt  time.Time     `bson:"start_at" json:"start_at" yaml:"start_at"`
	Duration time.Duration `bson:"duration" json:"duration" yaml:"duration"`
	env      sink.Environment
}

func NewBuildCostReport(env sink.Environment, name string, startAt time.Time, duration time.Duration) amboy.Job {
	j := makeBuildCostReport()

	j.env = env
	j.StartAt = startAt
	j.Duration = duration
	j.SetID(name)
	return j
}

func (j *buildCostReportJob) Run() {
	defer j.MarkComplete()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	grip.Infoln("would run build cost reporting job:", j.ID())

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
	output, err := cost.CreateReport(ctx, j.StartAt, j.Duration, costConf)
	if err != nil {
		grip.Warning(err)
		j.AddError(errors.WithStack(err))
		return
	}

	if err := output.Save(); err != nil {
		grip.Warning(err)
		j.AddError(err)
		return
	}

	summary := model.NewCostReportSummary(output)
	if err := summary.Save(); err != nil {
		grip.Warning(err)
		j.AddError(err)
		return
	}

	grip.Notice(message.Fields{
		"id":      "build-cost-report",
		"state":   "output",
		"period":  j.Duration.String(),
		"starts":  j.StartAt,
		"summary": summary,
	})
}
