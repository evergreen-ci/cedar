package operations

import "github.com/urfave/cli"

func Client() cli.Command {
	return cli.Command{
		Name:   "client",
		Usasge: "run a simple sink client",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "service",
				Usage:  "specify the URI of the sink service",
				EnvVar: "SINK_SERVICE_URL",
				Value:  "http://localhost:3000",
			},
		},
		Subcommands: []cli.Command{
			postSimpleLog(),
		},
	}
}

func postSimpleLog() cli.Command {
	return cli.Command{
		Name:  "simple-log-pipe",
		Usage: "posts a string",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name: "logId",
			},
		},
		Action: func(c *cli.Context) error {

		},
	}
}
