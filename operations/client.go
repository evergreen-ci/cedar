package operations

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/evergreen-ci/cedar/rest"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

// Client returns the entry point for the ./cedar client sub-command,
// which itself hosts a number of sub-commands. This client relies on
// an accessible cedar service.
func Client() cli.Command {
	return cli.Command{
		Name:   "client",
		Usage:  "run a simple cedar client",
		Flags:  restServiceFlags(),
		Before: mergeBeforeFuncs(requireClientHostFlag, requireClientPortFlag),
		Subcommands: []cli.Command{
			printStatus(),
			postSimpleLog(),
			getSimpleLog(),
			getSystemStatusEvents(),
			systemEvent(),
			systemInfo(),
		},
	}
}

func printStatus() cli.Command {
	return cli.Command{
		Name:  "status",
		Usage: "prints json document for the status of the service",
		Action: func(c *cli.Context) error {
			ctx := context.Background()

			opts := rest.ClientOptions{
				Host:   c.Parent().String(clientHostFlag),
				Port:   c.Parent().Int(clientPortFlag),
				Prefix: "",
			}
			client, err := rest.NewClient(opts)
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
		Name:   "simple-log-pipe",
		Usage:  "posts a string",
		Flags:  simpleLogIDFlags(),
		Before: requireFileExists(simpleLogIDFlag),
		Action: func(c *cli.Context) error {
			ctx := context.Background()

			opts := rest.ClientOptions{
				Host:   c.Parent().String(clientHostFlag),
				Port:   c.Parent().Int(clientPortFlag),
				Prefix: "",
			}
			client, err := rest.NewClient(opts)
			if err != nil {
				return errors.Wrap(err, "problem creating REST client")
			}

			logID := c.String(simpleLogIDFlag)
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
		Name:   "get-simple-log",
		Usage:  "prints json document for the simple log",
		Flags:  simpleLogIDFlags(),
		Before: requireFileExists(simpleLogIDFlag),
		Action: func(c *cli.Context) error {
			ctx := context.Background()
			logID := c.String(simpleLogIDFlag)

			opts := rest.ClientOptions{
				Host:   c.Parent().String(clientHostFlag),
				Port:   c.Parent().Int(clientPortFlag),
				Prefix: "",
			}
			client, err := rest.NewClient(opts)
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
		Usage: "prints json for all system events of a specified level",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "level",
				Usage: "specify a filter to a level for messages",
			},
			cli.IntFlag{
				Name:  "limit",
				Usage: "specify a number of messages to retrieve, defaults to no limit",
				Value: -1,
			},
		},
		Action: func(c *cli.Context) error {
			ctx := context.Background()

			opts := rest.ClientOptions{
				Host:   c.Parent().String(clientHostFlag),
				Port:   c.Parent().Int(clientPortFlag),
				Prefix: "",
			}
			client, err := rest.NewClient(opts)
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

func systemEvent() cli.Command {
	const (
		idFlag  = "id"
		ackFlag = "acknowledge"
	)

	return cli.Command{
		Name:  "system-event",
		Usage: "prints json for a specific system event",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  idFlag,
				Usage: "specify the Id of a log message",
			},
			cli.BoolFlag{
				Name:  ackFlag,
				Usage: "acknowledge the alert when specified",
			},
		},
		Action: func(c *cli.Context) error {
			ctx := context.Background()

			opts := rest.ClientOptions{
				Host:   c.Parent().String(clientHostFlag),
				Port:   c.Parent().Int(clientPortFlag),
				Prefix: "",
			}
			client, err := rest.NewClient(opts)
			if err != nil {
				return errors.Wrap(err, "problem creating REST client")
			}

			id := c.String(idFlag)

			var resp *rest.SystemEventResponse

			if c.Bool(ackFlag) {
				resp, err = client.AcknowledgeSystemEvent(ctx, id)
			} else {
				resp, err = client.GetSystemEvent(ctx, id)
			}

			if err != nil {
				return errors.Wrap(err, "problem with system event request")
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

func systemInfo() cli.Command {
	return cli.Command{
		Name:  "sysinfo",
		Usage: "save and access systems utilization metrics information",
		Subcommands: []cli.Command{
			systemInfoSend(),
			systemInfoImport(),
			systemInfoGet(),
		},
	}
}

func systemInfoGet() cli.Command {
	host, err := os.Hostname()
	grip.Warning(err)

	return cli.Command{
		Name:  "get",
		Usage: "returns system info documents for a host",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "host",
				Usage: "specify name of host",
				Value: host,
			},
			cli.StringFlag{
				Name:  "start",
				Usage: "RFC3339 formatted time. defaults to 24 hours ago",
				Value: time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
			},
			cli.StringFlag{
				Name:  "end",
				Usage: "RFC3339 formatted time. defaults to current time",
				Value: time.Now().Format(time.RFC3339),
			},
			cli.IntFlag{
				Name:  "limit",
				Usage: "number of results to return. defaults to no limit",
				Value: -1,
			},
		},
		Before: mergeBeforeFuncs(requireStringFlag("host")),
		Action: func(c *cli.Context) error {
			ctx := context.Background()

			opts := rest.ClientOptions{
				Host:   c.Parent().String(clientHostFlag),
				Port:   c.Parent().Int(clientPortFlag),
				Prefix: "",
			}
			client, err := rest.NewClient(opts)
			if err != nil {
				return errors.Wrap(err, "problem creating REST client")
			}

			catcher := grip.NewCatcher()
			start, err := time.Parse(time.RFC3339, c.String("start"))
			catcher.Add(err)
			end, err := time.Parse(time.RFC3339, c.String("end"))
			catcher.Add(err)
			if catcher.HasErrors() {
				return errors.Wrap(err, "problem pasring dates")
			}

			msgs, err := client.GetSystemInformation(ctx, c.String("host"), start, end, c.Int("limit"))
			if err != nil {
				return errors.WithStack(err)
			}

			out, err := pretyJSON(msgs)
			if err != nil {
				return errors.WithStack(err)
			}

			fmt.Println(out)
			return nil
		},
	}
}

func systemInfoSend() cli.Command {
	return cli.Command{
		Name:  "send",
		Usage: "collects and sends a system information document to the remote service",
		Action: func(c *cli.Context) error {
			ctx := context.Background()

			opts := rest.ClientOptions{
				Host:   c.Parent().String(clientHostFlag),
				Port:   c.Parent().Int(clientPortFlag),
				Prefix: "",
			}
			client, err := rest.NewClient(opts)
			if err != nil {
				return errors.Wrap(err, "problem creating REST client")
			}

			msg := message.CollectSystemInfo().(*message.SystemInfo)

			resp, err := client.SendSystemInfo(ctx, msg)
			if err != nil {
				return errors.Wrap(err, "problem sending system info")
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

func systemInfoImport() cli.Command {
	return cli.Command{
		Name:   "import",
		Usage:  "import system info data from a json file, one line per document",
		Flags:  addPathFlag(),
		Before: requireFileExists(pathFlagName),
		Action: func(c *cli.Context) error {
			ctx := context.Background()

			opts := rest.ClientOptions{
				Host:   c.Parent().String(clientHostFlag),
				Port:   c.Parent().Int(clientPortFlag),
				Prefix: "",
			}
			client, err := rest.NewClient(opts)
			if err != nil {
				return errors.Wrap(err, "problem creating REST client")
			}

			fn := c.String(pathFlagName)
			f, err := os.Open(fn)
			if err != nil {
				return errors.Wrapf(err, "problem opening file '%s'", fn)
			}
			defer f.Close()
			r := bufio.NewReader(f)

			catcher := grip.NewCatcher()
			var count int
			var line []byte
			for {
				ln, prefix, err := r.ReadLine()
				if err == io.EOF {
					break
				} else if err != nil {
					return errors.Wrap(err, "problem reading file: "+fn)
				}

				count++
				if prefix {
					line = append(line, ln...)
					continue
				}

				if len(line) > 0 {
					msg := &message.SystemInfo{}

					if err = json.Unmarshal(line, msg); err != nil {
						catcher.Add(err)
						continue
					}
					var resp *rest.SystemInfoReceivedResponse
					resp, err = client.SendSystemInfo(ctx, msg)
					grip.Debugf("%+v", resp)
					if err != nil {
						grip.Warning(err)
						grip.Alert(resp.Error)
						return errors.Wrap(err, "problem sending data")
					}
					line = []byte{}
				}

				msg := &message.SystemInfo{}
				if err = json.Unmarshal(ln, msg); err != nil {
					catcher.Add(err)
					continue
				}

				resp, err := client.SendSystemInfo(ctx, msg)
				grip.Debugf("%+v", resp)
				if err != nil {
					grip.Warning(err)
					grip.Alert(resp.Error)
					return errors.Wrap(err, "problem sending data")
				}
			}

			return catcher.Resolve()
		},
	}
}
