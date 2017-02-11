package units

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimpleParser(t *testing.T) {
	assert := assert.New(t)

	parser := &parseSimpleLogJobName{Key: "foo", Content: []string{"foo", "bar"}}
	assert.NoError(parser.Validate())
	parser.Run()
	assert.True(parser.Completed())
	assert.NoError(parser.Error())

	assert.Equal(parser.numLines, 2)
	assert.Equal(parser.freq["o"], 2)
}
