package operations

import (
	"testing"

	"github.com/mongodb/grip"
	"github.com/stretchr/testify/suite"
	"github.com/urfave/cli"
)

func init() {
	grip.SetName("cedar.operations.test")
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
	cmd := Cost()
	s.Len(cmd.Flags, 0)
	cmd = write()
	s.Len(cmd.Flags, 8)
	for _, flag := range cmd.Flags {

		switch flag.GetName() {
		case "start", "config":
			s.IsType(cli.StringFlag{}, flag)
		case "duration":
			s.IsType(cli.DurationFlag{}, flag)
		case "continue-on-error", "continueOnError", "disableEvgAll", "disableEvgProjects", "disableEvgDistros":
			s.IsType(cli.BoolFlag{}, flag)
		}

	}
}
