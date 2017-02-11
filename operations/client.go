package operations

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

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
			getSimpleLog(),
			getSystemStatusEvents(),
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
			out, err := pretyJSON(status)
			if err != nil {
				return errors.WithStack(err)
			}

			fmt.Println(out)
			return nil
		},
	}

}

func pretyJSON(data interface{}) (string, error) {
	out, err := json.MarshalIndent(data, "", "   ")
	if err != nil {
		return "", errors.Wrap(err, "problem rendering status result")
	}

	return string(out), nil
}

func postSimpleLog() cli.Command {
	return cli.Command{
		Name:  "simple-log-pipe",
		Usage: "posts a string",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "log",
				Usage: "identifier for the log",
			},
		},
		Action: func(c *cli.Context) error {
			ctx := context.Background()

			client, err := rest.NewClient(c.Parent().String("host"), c.Parent().Int("port"), "")
			if err != nil {
				return errors.Wrap(err, "problem creating REST client")
			}

			logID := c.String("log")
			inc := 0
			batch := []string{}

			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				batch = append(batch, scanner.Text())
				if len(batch) >= 100 {
					resp, err := client.WriteSimpleLog(ctx, logID, strings.Join(batch, "\n"), inc)
					if err != nil {
						return errors.Wrapf(err, "problem sending batch %d for log '%s'",
							inc, logID)
					}
					respRendered, err := pretyJSON(resp)
					if err != nil {
						return errors.WithStack(err)
					}
					grip.Infof("posted batch %d of log %s: %s",
						inc, logID, respRendered)
					batch = []string{}
					inc++
				}
			}

			if len(batch) > 0 {
				// post one final do one final batch
				resp, err := client.WriteSimpleLog(ctx, logID, strings.Join(batch, "\n"), inc)
				if err != nil {
					return errors.Wrapf(err, "problem sending batch %d for log '%s'",
						inc, logID)
				}
				respRendered, err := pretyJSON(resp)
				if err != nil {
					return errors.WithStack(err)
				}
				grip.Infof("posted final batch %d of log %s: %s",
					inc, logID, respRendered)
			}

			return nil
		},
	}
}

func getSimpleLog() cli.Command {
	return cli.Command{
		Name:  "get-simple-log",
		Usage: "prints json document for the simple log",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "log",
				Usage: "identifier for the log",
			},
		},
		Action: func(c *cli.Context) error {
			ctx := context.Background()
			logID := c.String("log")

			client, err := rest.NewClient(c.Parent().String("host"), c.Parent().Int("port"), "")
			if err != nil {
				return errors.Wrap(err, "problem creating REST client")
			}

			resp, err := client.GetSimpleLog(ctx, logID)
			if err != nil {
				return errors.Wrapf(err, "problem getting log for '%s'", logID)
			}
			grip.Debug(resp)
			out, err := pretyJSON(resp)
			if err != nil {
				return errors.WithStack(err)
			}

			fmt.Println(out)
			return nil
		},
	}

}

func getSystemStatusEvents() cli.Command {
	return cli.Command{
		Name:  "get-system-events",
		Usage: "writes ",
		Flags: []cli.Flag{
			cli.StringFlag{
				Ndame: "level",
				Usage: "specify a filter to a level for messages",
			},
			cli.Intflag{
				Name:  "limit",
				Usage: "specify a number of messages to retrieve, defaults to no limit",
				Value: -1,
			},
		},
		Action: func(c *cli.Context) error {
			ctx := context.Background()

			client, err := rest.NewClient(c.Parent().String("host"), c.Parent().Int("port"), "")
			if err != nil {
				return errors.Wrap(err, "problem creating REST client")
			}

			resp, err := client.GetSystemEvents(ctx, c.String("level"), c.Int("limit"))
			if err != nil {
				return errors.Wrap(err, "problem getting system event log")
			}

			grip.Debug(resp)
			out, err := pretyJSON(resp)
			if err != nil {
				return errors.WithStack(err)
			}

			fmt.Println(out)
			return nil
		},
	}

}
