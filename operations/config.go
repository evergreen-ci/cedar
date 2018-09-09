package operations

import (
	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/model"
	"github.com/evergreen-ci/sink/util"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

// Config is responsible for managing application level configuration
// (logging, external services, etc.) which is saved in the database.
func Config() cli.Command {
	return cli.Command{
		Name:  "conf",
		Usage: "sink application configuration",
		Subcommands: []cli.Command{
			loadSinkConfig(),
			dumpSinkConfig(),
		},
	}
}

func dumpSinkConfig() cli.Command {
	return cli.Command{
		Name:  "dump-config",
		Usage: "write current sink application configuration to a file",
		Flags: dbFlags(
			cli.StringFlag{
				Name:  "file",
				Usage: "specify path to a sink application config file",
			}),
		Action: func(c *cli.Context) error {
			env := sink.GetEnvironment()

			fileName := c.String("file")
			mongodbURI := c.String(dbURIFlag)
			dbName := c.String(dbNameFlag)

			if err := configure(env, 2, true, mongodbURI, "", dbName); err != nil {
				return errors.WithStack(err)
			}

			conf := &model.SinkConfig{}
			conf.Setup(env)

			if err := conf.Find(); err != nil {
				return errors.WithStack(err)
			}

			return errors.WithStack(util.WriteJSON(fileName, conf))
		},
	}
}

func loadSinkConfig() cli.Command {
	return cli.Command{
		Name:  "load-config",
		Usage: "loads sink application configuration from a file",
		Flags: dbFlags(
			cli.StringFlag{
				Name:  "file",
				Usage: "specify path to a sink application config file",
			}),
		Action: func(c *cli.Context) error {
			env := sink.GetEnvironment()

			fileName := c.String("file")
			mongodbURI := c.String(dbURIFlag)
			dbName := c.String(dbNameFlag)

			if err := configure(env, 2, true, mongodbURI, "", dbName); err != nil {
				return errors.WithStack(err)
			}

			conf, err := model.LoadSinkConfig(fileName)
			if err != nil {
				return errors.WithStack(err)
			}
			conf.Setup(env)

			if err = conf.Save(); err != nil {
				return errors.WithStack(err)
			}

			grip.Infoln("successfully application configuration to database at:", mongodbURI)
			return nil
		},
	}
}
