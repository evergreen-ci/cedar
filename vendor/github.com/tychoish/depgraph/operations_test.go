package depgraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTypeSlice(t *testing.T) {
	assert := assert.New(t)

	assert.True(typeSliceIs([]int{1, 2, 4}, 4))
	assert.False(typeSliceIs([]int{1, 2, 4}, 5))
}

func TestCleanNameForDot(t *testing.T) {
	assert := assert.New(t)

	assert.Equal(`"foo\"bar"`, cleanNameForDot(`foo"bar`))
}
