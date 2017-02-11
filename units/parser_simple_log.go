package units

import (
	"strings"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/pkg/errors"
	"github.com/tychoish/grip"
	"github.com/tychoish/sink/model"
)

const (
	parseSimpleLogJobName = "simple-log-parse"
	letters               = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

// parseSimpleLog parses simple log content
type parseSimpleLog struct {
	Key       string   `bson:"logID" json:"logID" yaml:"logID"`
	Content   []string `bson:"content" json:"content" yaml:"content"`
	*job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`

	// TODO persist this somehow
	freq     map[string]int
	numLines int
}

func init() {
	registry.AddJobType(parseSimpleLogJobName, func() amboy.Job {
		sp := &parseSimpleLog{
			Base: &job.Base{
				JobType: amboy.JobType{
					Name:    parseSimpleLogJobName,
					Version: 1,
				},
			},
		}
		sp.SetDependency(dependency.NewAlways())

		return sp
	})
}

func (sp *parseSimpleLog) Validate() error {
	if sp.Key == "" {
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

func (sp *parseSimpleLog) reset() {
	sp.Content = []string{}
}

// Parse takes the log id
func (sp *parseSimpleLog) Run() {
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

	sp.numLines = len(sp.Content)
	// TODO shouldn't this count new line characters rather than characters?
	if err := l.SetNumberLines(sp.numLines); err != nil {
		err = errors.Wrap(err, "problem setting metadata")
		grip.Warning(err)
		sp.AddError(err)
	}
}
