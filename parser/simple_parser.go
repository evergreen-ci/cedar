package parser

import (
	"github.com/tychoish/sink/model/log"
)

type Parser interface {
	// Parse takes in a string slice
	Parse(id string, s []string) error
}

// SimpleParser implements the Parser interface
type SimpleParser struct{}

func (sp *SimpleParser) Parse(id string, content []string) error {
	l, err := log.FindOne(log.ById(id))
	if err != nil {
		return nil, err
	}
	return l.SetNumberLines(len(content))
}
