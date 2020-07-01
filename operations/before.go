package operations

import (
	"os"

	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

// this package contains validator functions passed to command and
// subcommand functions to check the contents of flags. There are a
// number of function literals in variable names which provide
// specific validation for flags, as well as some more generic helper
// functions to generate and define these.

var (
	requireClientHostFlag = func(c *cli.Context) error {
		if c.Parent().String(clientHostFlag) == "" {
			return errors.New("host not specified for client")
		}
		return nil
	}

	requireClientPortFlag = func(c *cli.Context) error {
		// TODO consider validating
		if c.Parent().Int(clientPortFlag) == 0 {
			return errors.New("port not specified for client")
		}
		return nil
	}
)

func requireStringFlag(name string) cli.BeforeFunc {
	return func(c *cli.Context) error {
		if c.String(name) == "" {
			return errors.Errorf("flag '--%s' was not specified", name)
		}
		return nil
	}
}

func requireOneFlag(names ...string) cli.BeforeFunc {
	return func(c *cli.Context) error {
		var numSet int
		for _, name := range names {
			if c.IsSet(name) {
				numSet++
			}
		}
		if numSet != 1 {
			return errors.Errorf("must set exacty one flag from the following: %s", names)
		}
		return nil
	}
}

func requireFileExists(name string) cli.BeforeFunc {
	return func(c *cli.Context) error {
		path := c.String(name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return errors.Errorf("file '%s' does not exist", path)
		}

		return nil
	}
}

func mergeBeforeFuncs(ops ...func(c *cli.Context) error) cli.BeforeFunc {
	return func(c *cli.Context) error {
		catcher := grip.NewBasicCatcher()

		for _, op := range ops {
			catcher.Add(op(c))
		}

		return catcher.Resolve()
	}
}
