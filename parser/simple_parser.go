package parser

import (
	"strings"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/pkg/errors"
	"github.com/tychoish/grip"
	"github.com/tychoish/sink/db"
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
	Name      string   `bson:"_id" json:"id" yaml:"id"`
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

func (sp *SimpleLog) SetID(n string) { sp.Base.SetID(n) }
func (sp *SimpleLog) SetOptions(opts interface{}) error {
	if opts != nil {
		input, ok := opts.(*SimpleLog)
		if !ok {
			return errors.Errorf("options is %T not %T", input, sp)
		}
		sp = input
	}

	if sp.Name == "" {
		return errors.New("no id given")
	}

	if len(sp.Content) == 0 {
		return errors.New("no content")
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
	defer sp.reset()

	l := &model.Log{}

	if err := l.Find(db.IDQuery(sp.Name)); err != nil {
		sp.AddError(errors.Wrap(err, "problem running query"))
		return
	}

	for i := 0; i <= len(sp.Content)+1; i++ {
		char := sp.Content[i]

		if strings.Contains(char, letters) {
			total := sp.freq[char]
			total++
			sp.freq[char] = total
		}
	}
	grip.Infof("letter frequencies: %+v", sp.freq)

	// TODO shouldn't this count new line characters rather than characters?
	if err := l.SetNumberLines(len(sp.Content)); err != nil {
		sp.AddError(errors.Wrap(err, "problem setting metadata"))
	}
}
