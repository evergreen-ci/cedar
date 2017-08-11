package operations

import (
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"github.com/tychoish/depgraph"
	"github.com/urfave/cli"
)

// Deps exposes (or will expose) client functionality for the
// dependency information graph.
func Deps() cli.Command {
	return cli.Command{
		Name: "deps",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "path",
				Value: "deps.json",
			},
		},
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
