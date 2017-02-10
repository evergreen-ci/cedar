package parser

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/tychoish/grip"
	"github.com/tychoish/sink/model"
)

const (
	letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

// ParserOptions is all possible inputs to parsers
type SimpleParserOptions struct {
	ID      string   `bson:"_id" json:"id" yaml:"id"`
	Content []string `bson:"content" json:"content" yaml:"content"`

	// TODO persist this somehow
	freq map[string]int
}

type Parser interface {
	Initialize(opts interface{}) error
	// Parse takes in a string slice
	Parse() error

	Reset()
}

// SimpleParser implements the Parser interface
type SimpleParser struct {
	opts *SimpleParserOptions
}

func (sp *SimpleParser) Initialize(opts interface{}) error {
	spOpts, ok := opts.(*SimpleParserOptions)
	if !ok {
		return errors.Errorf("options is %T not %T", opts, sp.opts)
	}
	sp.opts = spOpts

	if sp.opts.ID == "" {
		return errors.New("no id given")
	}
	if len(sp.opts.Content) == 0 {
		return errors.New("no content")
	}

	if sp.opts.freq == nil {
		sp.opts.freq = map[string]int{}
	}

	return nil
}

func (sp *SimpleParser) Reset() {
	sp.opts = &SimpleParserOptions{}
}

// Parse takes the log id
func (sp *SimpleParser) Parse() error {
	l, err := model.FindOneLog(model.ByID(sp.opts.ID))
	if err != nil {
		return err
	}

	for i := 0; i <= len(sp.opts.Content)+1; i++ {
		char := sp.opts.Content[i]

		if strings.Contains(char, letters) {
			total := sp.opts.freq[char]
			total++
			sp.opts.freq[char] = total
		}
	}
	grip.Infof("letter frequencies: %+v", sp.opts.freq)

	// TODO shouldn't this count new line characters
	return l.SetNumberLines(len(sp.opts.Content))
}
