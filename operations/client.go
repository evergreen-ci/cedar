package operations

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/tychoish/grip"
	"github.com/tychoish/sink/rest"
	"github.com/urfave/cli"
)

func Client() cli.Command {
	return cli.Command{
		Name:  "client",
		Usage: "run a simple sink client",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "host",
				Usage: "host for the remote greenbay instance.",
				Value: "http://localhost",
			},
			cli.IntFlag{
				Name:  "port",
				Usage: "port for the remote greenbay service.",
				Value: 3000,
			},
		},
		Subcommands: []cli.Command{
			printStatus(),
			postSimpleLog(),
		},
	}
}

func printStatus() cli.Command {
	return cli.Command{
		Name:  "status",
		Usage: "prints json document for the status of the service",
		Action: func(c *cli.Context) error {
			ctx := context.Background()

			client, err := rest.NewClient(c.Parent().String("host"), c.Parent().Int("port"), "")
			if err != nil {
				return errors.Wrap(err, "problem creating REST client")
			}

			status, err := client.GetStatus(ctx)
			if err != nil {
				return errors.Wrap(err, "problem getting status")
			}

			grip.Debug(status)
			out, err := json.MarshalIndent(status, "", "   ")
			if err != nil {
				return errors.Wrap(err, "problem rendering status result")
			}

			fmt.Println(string(out))
			return nil
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
			return nil
		},
	}
}
