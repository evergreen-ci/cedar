package parser

import (
	"fmt"
	"time"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/pkg/errors"
	"github.com/tychoish/sink/model/log"
)

var (
	parserJobName = "parse-simple-log"
)

type Parser interface {
	// Parse takes in a string slice
	Parse() error
}

// SimpleParser implements the Parser interface
type SimpleParser struct {
	Id      string
	Content []string
}

// Parse takes the log id
func (sp *SimpleParser) Parse() error {
	l, err := log.FindOne(log.ById(sp.Id))
	if err != nil {
		return err
	}
	return l.SetNumberLines(len(sp.Content))
}

type saveSimpleParserToDBJob struct {
	Timestamp time.Time `bson:"ts" json:"ts" yaml:"timestamp"`
	Content   []string  `bson:"content" json:"content" yaml:"content"`
	Increment int       `bson:"i" json:"inc" yaml:"increment"`
	LogId     string    `bson:"logId" json:"logId" yaml:"logId"`
	*job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

func saveSimpleParserToDbJobFactory() amboy.Job {
	j := &saveSimpleParserToDBJob{
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
func MakeParserJob(logId string, content []string, ts time.Time, inc int) amboy.Job {
	j := saveSimpleParserToDbJobFactory().(*saveSimpleParserToDBJob)
	j.SetID(fmt.Sprintf("%s-%s-%d", j.Type(), logId, inc))
	j.Timestamp = ts
	j.Content = content
	j.LogId = logId
	j.Increment = inc

	return j
}

func (j *saveSimpleParserToDBJob) Run() {
	defer j.MarkComplete()
	p := SimpleParser{
		Id:      j.LogId,
		Content: j.Content,
	}
	err := p.Parse()
	if err != nil {
		j.AddError(errors.Wrap(err, "error parsing content"))
	}
}
