package operations

import (
	"encoding/json"
	"os"

	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"github.com/tychoish/depgraph"
	"github.com/urfave/cli"
)

// Deps exposes (or will expose) client functionality for the
// mongodb server library, file, and symbol dependency graph.
func Dagger() cli.Command {
	return cli.Command{
		Name:  "dagger",
		Usage: "access mongodb library dependency information",
		Subcommands: []cli.Command{
			smoke(),
			filterLibrary(),
		},
	}
}

func smoke() cli.Command {
	return cli.Command{
		Name:  "smoke",
		Usage: "load a graph to perform a smoke test",
		Flags: depsFlags(),
		Action: func(c *cli.Context) error {
			fn := c.String("path")
			grip.Infoln("starting to load graph from:", fn)
			graph, err := depgraph.New("cli", fn)
			if err != nil {
				return errors.Wrap(err, "problem loading graph")
			}

			grip.Infof("first node: %+v", graph.Nodes[0])
			grip.Infof("first edge: %+v", graph.Edges[0])

			return nil
		},
	}
}

func filterLibrary() cli.Command {
	return cli.Command{
		Name:  "filter-library",
		Usage: "take a full graph and return only the library components",
		Flags: depsFlags(cli.StringFlag{
			Name:  "output",
			Value: "libraryDeps.json",
			Usage: "specify the path to the filtered library graph",
		}),
		Action: func(c *cli.Context) error {
			fn := c.String("path")
			grip.Infoln("starting to load graph from:", fn)
			graph, err := depgraph.New("cli", fn)
			if err != nil {
				return errors.Wrap(err, "problem loading graph")
			}

			// make graph library only
			libgraph := graph.Filter(depgraph.LibraryToLibrary, depgraph.Library)

			return errors.Wrap(writeJSON(c.String("output"), libgraph),
				"problem writing filtered graph")
		},
	}

}

func writeJSON(fn string, data interface{}) error {
	out, err := json.MarshalIndent(data, "", "   ")
	if err != nil {
		return errors.Wrap(err, "problem writing libgraph to disk")
	}

	f, err := os.Create(fn)
	if err != nil {
		return errors.WithStack(err)
	}
	defer f.Close()

	out = append(out, []byte("\n")...)
	if _, err = f.Write(out); err != nil {
		return err
	}

	if err = f.Sync(); err != nil {
		return err
	}

	return nil
}
