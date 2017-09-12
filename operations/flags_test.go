package operations

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli"
)

func TestBaseFlags(t *testing.T) {
	assert := assert.New(t)

	flags := baseFlags(dbFlags())
	flagMap := map[string]cli.Flag{}
	for _, f := range flags {
		flagMap[f.GetName()] = f
	}

	expected := []string{"workers", "dbUri", "dbName", "bucket"}
	for _, n := range expected {
		_, ok := flagMap[n]
		assert.True(ok, n)
	}
}
