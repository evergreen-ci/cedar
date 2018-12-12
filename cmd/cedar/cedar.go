package main

import (
	"os"

	"github.com/evergreen-ci/cedar/operations"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

func main() {
	// this is where the main action of the program starts. The
	// command line interface is managed by the cli package and
	// its objects/structures. This, plus the basic configuration
	// in buildApp(), is all that's necessary for bootstrapping the
	// environment.
	app := buildApp()
	err := app.Run(os.Args)
	grip.CatchEmergencyFatal(err)
}

func buildApp() *cli.App {
	app := cli.NewApp()

	app.Name = "cedar"
	app.Usage = "a data processing API"
	app.Version = "0.0.1-pre"

	app.Commands = []cli.Command{
		operations.Admin(),
		operations.Service(),
		operations.Client(),
		operations.Worker(),
		operations.Dagger(),
		operations.Cost(),
	}

	// These are global options. Use this to configure logging or
	// other options independent from specific sub commands.
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "level",
			Value: "info",
			Usage: "Specify lowest visible loglevel as string: 'emergency|alert|critical|error|warning|notice|info|debug'",
		},
	}

	app.Before = func(c *cli.Context) error {
		return errors.WithStack(loggingSetup(app.Name, c.String("level")))
	}

	return app
}

// logging setup is separate to make it unit testable
func loggingSetup(name, logLevel string) error {
	sender := grip.GetSender()
	sender.SetName(name)

	lvl := sender.Level()
	lvl.Threshold = level.FromString(logLevel)
	return errors.WithStack(sender.SetLevel(lvl))
}
