package depgraph

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDotGenerator(t *testing.T) {
	assert := assert.New(t)
	report := GraphMap{}
	assert.Len(report, 0)
	assert.Len(strings.Split(report.Dot(), "\n"), 4)

	report["foo"] = []string{}
	assert.Len(report, 1)
	assert.Len(strings.Split(report.Dot(), "\n"), 5)

	report["foo"] = []string{"a", "b"}
	report["a"] = []string{}
	report["b"] = []string{}
	assert.Len(strings.Split(report.Dot(), "\n"), 9)
}
