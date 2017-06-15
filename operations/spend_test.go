package operations

import (
	"testing"

	"github.com/mongodb/grip"
	"github.com/stretchr/testify/suite"
	"github.com/urfave/cli"
)

func init() {
	grip.SetName("sink.operations.test")

}

// CommandsSuite provide a group of tests of the entry points and
// command wrappers for command-line interface to curator.
type CommandsSuite struct {
	suite.Suite
}

func TestCommandSuite(t *testing.T) {
	suite.Run(t, new(CommandsSuite))
}

func (s *CommandsSuite) TestSpendFlags() {
	cmd := Spend()
	s.Len(cmd.Flags, 2)
	for _, flag := range cmd.Flags {
		name := flag.GetName()

		if name == "start" {
			s.IsType(cli.StringFlag{}, flag)
		} else {
			s.Equal(name, "granularity")
			s.IsType(cli.DurationFlag{}, flag)
		}
	}
}

//Further tests will be written after spend functionality is written
