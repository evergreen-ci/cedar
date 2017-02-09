package units

import (
	"fmt"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/pkg/errors"
	"github.com/tychoish/sink/parser"
)

const (
	parseSimpleLogJobName = "parse-simple-log"
)

func init() {
	registry.AddJobType(parseSimpleLogJobName, func() amboy.Job {
		return saveParserToDBJobFactory()
	})
}

type saveParserToDBJob struct {
	Parser    parser.Parser        `bson:"parser" json:"parser" yaml:"parser"`
	Opts      parser.ParserOptions `bson:"opts" json:"opts" yaml:"opts"`
	*job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

func saveParserToDBJobFactory() amboy.Job {
	j := &saveParserToDBJob{
		Base: &job.Base{
			JobType: amboy.JobType{
				Name:    parseSimpleLogJobName,
				Version: 1,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func MakeParserJob(parser parser.Parser, opts parser.ParserOptions) amboy.Job {
	// create the parser with the log id and content
	j := saveParserToDBJobFactory().(*saveParserToDBJob)
	j.SetID(fmt.Sprintf("%s-%s", j.Type(), opts.Id))
	j.Parser = parser
	j.Opts = opts
	return j
}

func (j *saveParserToDBJob) Run() {
	defer j.MarkComplete()
	err := j.Parser.Initialize(j.Opts)
	if err != nil {
		j.AddError(errors.Wrap(err, "error initializing parser"))
		return
	}
	err = j.Parser.Parse()
	if err != nil {
		j.AddError(errors.Wrap(err, "error parsing content"))
	}
}
