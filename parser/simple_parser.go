package parser

import (
	"fmt"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/pkg/errors"
	"github.com/tychoish/sink/model/log"
)

const (
	parserJobName = "parse-simple-log"
)

// ParserOptions is all possible inputs to parsers
type ParserOptions struct {
	Id      string
	Content []string
}

type Parser interface {
	Initialize(opts *ParserOptions) error
	// Parse takes in a string slice
	Parse() error
}

// SimpleParser implements the Parser interface
type SimpleParser struct {
	Id      string
	Content []string
}

func (sp *SimpleParser) Initialize(opts *ParserOptions) error {
	if opts.Id == "" {
		return errors.New("no id given")
	}
	if len(opts.Content) == 0 {
		return errors.New("no content")
	}
	sp.Id = opts.Id
	sp.Content = opts.Content

	return nil
}

// Parse takes the log id
func (sp *SimpleParser) Parse() error {
	l, err := log.FindOne(log.ById(sp.Id))
	if err != nil {
		return err
	}
	return l.SetNumberLines(len(sp.Content))
}

type saveParserToDBJob struct {
	Parser    Parser         `bson:"parser" json:"parser" yaml:"parser"`
	Opts      *ParserOptions `bson:"opts" json:"opts" yaml:"opts"`
	*job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

func saveSimpleParserToDbJobFactory() amboy.Job {
	j := &saveParserToDBJob{
		Base: &job.Base{
			JobType: amboy.JobType{
				Name:    parserJobName,
				Version: 1,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}
func MakeParserJob(parser Parser, opts *ParserOptions) amboy.Job {
	// create the parser with the log id and content
	j := saveSimpleParserToDbJobFactory().(*saveParserToDBJob)
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
