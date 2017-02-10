package units

import (
	"fmt"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/pkg/errors"
	"github.com/tychoish/grip"
	"github.com/tychoish/sink/parser"
)

// ParserJobFactories take a name, and the options struct for a parser.
type ParserJobFactory func(string, interface{}) amboy.Job

var (
	MakeSimpleLogParserJob = makeParserJobFactory("parse-simple-log", &parser.SimpleParser{})
)

///////////////////////////////////
//
// Task Implementation

// parserJob provides a generic worker for running parser.Parser operations.
type parserJob struct {
	Opts      interface{} `bson:"opts" json:"opts" yaml:"opts"`
	*job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`

	parser parser.Parser
}

// Run is the only required method of the amboy.Job interface that is
// not implemented by *job.Base, and provides the entry point for the
// work unit proivded by the job.
func (j *parserJob) Run() {
	grip.Infof("running parser (%s): %s", j.Type().Name, j.ID())
	defer j.MarkComplete()

	if err := j.parser.Initialize(j.Opts); err != nil {
		err = errors.Wrap(err, "error initializing parser")
		grip.Error(err)
		j.AddError(err)
		return
	}

	defer j.parser.Reset()

	if err := j.parser.Parse(); err != nil {
		err = errors.Wrap(err, "error parsing content")
		grip.Warning(err)
		j.AddError(err)
		return
	}

	grip.Infof("completed parsing job %s", j.ID())
}

// makeParserJobFactory constructs and registers a job *and* returns a
// constructor/factory of type ParserJobFactory which you can to
// create jobs that use the parser.Parser interface.
func makeParserJobFactory(name string, p parser.Parser) ParserJobFactory {
	f := func() amboy.Job {
		j := &parserJob{
			parser: p,
			Base: &job.Base{
				JobType: amboy.JobType{
					Name:    name,
					Version: 1,
					Format:  amboy.JSON,
				},
			},
		}
		j.SetDependency(dependency.NewAlways())
		return j
	}
	registry.AddJobType(name, f)

	return func(id string, opts interface{}) amboy.Job {
		j := f().(*parserJob)
		j.SetID(fmt.Sprintf("%s-%s", j.Type().Name, id))
		j.Opts = opts

		return j
	}
}
