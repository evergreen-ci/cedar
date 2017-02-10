package parser

import (
	"github.com/pkg/errors"
	"github.com/tychoish/sink/model/log"
)

// ParserOptions is all possible inputs to parsers
type ParserOptions struct {
	ID      string
	Content []string
}

type Parser interface {
	Initialize(opts ParserOptions) error
	// Parse takes in a string slice
	Parse() error
}

// SimpleParser implements the Parser interface
type SimpleParser struct {
	ID      string
	Content []string
}

func (sp *SimpleParser) Initialize(opts ParserOptions) error {
	if opts.ID == "" {
		return errors.New("no id given")
	}
	if len(opts.Content) == 0 {
		return errors.New("no content")
	}
	sp.ID = opts.ID
	sp.Content = opts.Content

	return nil
}

// Parse takes the log id
func (sp *SimpleParser) Parse() error {
	l, err := log.FindOne(log.ByID(sp.ID))
	if err != nil {
		return err
	}
	return l.SetNumberLines(len(sp.Content))
}
