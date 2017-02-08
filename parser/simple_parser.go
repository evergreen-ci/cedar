package parser

type Parser interface {
	// Parse takes in a string slice
	Parse(s []string) error
	Save() error
}

// SimpleParser implements the Parser interface
type SimpleParser struct {
	NumberLines int
}

func (sp *SimpleParser) Parse(s []string) error {
	sp.NumberLines = len(s)
	return nil
}
