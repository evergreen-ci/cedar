package units

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimpleParser(t *testing.T) {
	assert := assert.New(t)

	parser := &parseSimpleLog{
		Key:     "foo",
		Content: []string{"foo", "bar"},
	}
	assert.NoError(parser.Validate())
	parser.Run(context.TODO())
}
