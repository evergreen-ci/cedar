package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimpleParser(t *testing.T) {
	assert := assert.New(t)

	parser := SimpleParser{}
	input := []string{"hello", "world", "blah"}
	assert.NoError(parser.Parse(input))
	assert.Equal(parser.NumberLines, 3)

}
