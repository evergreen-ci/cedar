package parser

import (
	"fmt"
	"strings"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/pkg/errors"
	"github.com/tychoish/grip"
	"github.com/tychoish/sink/model"
)

var (
	MakeSimpleLogUnit = RegisterParserUnit("parse-simple-log", simpleLogParserFactory)
)

const (
	letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

// SimpleLog parses simple log content
type SimpleLog struct {
	Key       string   `bson:"logID" json:"logID" yaml:"logID"`
	Content   []string `bson:"content" json:"content" yaml:"content"`
	*job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`

	// TODO persist this somehow
	freq map[string]int
}

func simpleLogParserFactory(name string) Parser {
	sp := &SimpleLog{
		Base: &job.Base{
			JobType: amboy.JobType{
				Name:    name,
				Version: 1,
			},
		},
	}
	sp.SetDependency(dependency.NewAlways())

	return sp
}

func (sp *SimpleLog) SetID(n string) { sp.Base.SetID(fmt.Sprintf("%s-%d", n, job.GetNumber())) }
func (sp *SimpleLog) SetOptions(opts interface{}) error {
	var input *SimpleLog
	var ok bool

	if opts != nil {
		input, ok = opts.(*SimpleLog)
		if !ok {
			return errors.Errorf("options is %T not %T", input, sp)
		}
	}

	if sp.Key == "" {
		if input == nil || input.Key == "" {
			return errors.New("no id given")
		}

		sp.Key = input.Key
	}

	if len(sp.Content) == 0 {
		if input == nil || len(input.Content) == 0 {
			return errors.New("no content")
		}

		sp.Content = input.Content
	}

	if sp.freq == nil {
		sp.freq = map[string]int{}
	}

	return nil
}

func (sp *SimpleLog) reset() {
	sp.Content = []string{}
}

// Parse takes the log id
func (sp *SimpleLog) Run() {
	defer sp.MarkComplete()
	defer sp.reset()

	l := &model.LogSegment{}

	if err := l.Find(model.ByLogID(sp.Key)); err != nil {
		err = errors.Wrap(err, "problem running query")
		grip.Warning(err)
		sp.AddError(err)
		return
	}

	if sp.freq == nil {
		sp.freq = map[string]int{}
	}

	for _, line := range sp.Content {
		for i := 0; i < len(line); i++ {
			char := string(line[i])

			if strings.Contains(letters, char) {
				total := sp.freq[char]
				total++
				sp.freq[char] = total
			}
		}

	}
	grip.Infof("letter frequencies: %+v", sp.freq)

	// TODO shouldn't this count new line characters rather than characters?
	if err := l.SetNumberLines(len(sp.Content)); err != nil {
		err = errors.Wrap(err, "problem setting metadata")
		grip.Warning(err)
		sp.AddError(err)
	}
}
