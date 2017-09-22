package operations

import (
	"github.com/evergreen-ci/sink"
	"github.com/evergreen-ci/sink/model"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
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
			loadGraphToDB(),
			cleanDB(),
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

func loadGraphToDB() cli.Command {
	return cli.Command{
		Name:  "load",
		Usage: "load a graph from a file into a local database",
		Flags: dbFlags(depsFlags()...),
		Action: func(c *cli.Context) error {
			mongodbURI := c.String("dbUri")
			dbName := c.String("dbName")
			fn := c.String("path")

			env := sink.GetEnvironment()

			if err := configure(env, 2, true, mongodbURI, "", dbName); err != nil {
				return errors.WithStack(err)
			}

			grip.Infoln("loading graph from:", fn)
			graph, err := depgraph.New("cli", fn)
			if err != nil {
				return errors.Wrap(err, "problem loading graph")
			}

			if graph.BuildID == "" {
				graph.BuildID = uuid.NewV4().String()
			}

			gdb := &model.GraphMetadata{
				BuildID: graph.BuildID,
			}
			gdb.Setup(env)
			if err := gdb.Insert(); err != nil {
				return errors.Wrapf(err, "problem saving root node for %s", graph.BuildID)
			}

			catcher := grip.NewSimpleCatcher()
			for _, node := range graph.Nodes {
				ndb := gdb.MakeNode(node)
				if err = ndb.Insert(); err != nil {
					catcher.Add(err)
					continue
				}
				grip.Debugf("adding node for '%s'", ndb.ID)
			}

			for _, edge := range graph.Edges {
				edb := gdb.MakeEdge(edge)
				if err = edb.Insert(); err != nil {
					catcher.Add(err)
					continue
				}
				grip.Debugf("adding edge for '%s'", edb.ID)
			}

			if catcher.HasErrors() {
				return catcher.Resolve()
			}
			if err = gdb.MarkComplete(); err != nil {
				return errors.WithStack(err)
			}

			grip.Info("adding graph '%s' with '%d' nodes and '%d' edges")

			return err
		},
	}
}

func cleanDB() cli.Command {
	return cli.Command{
		Name:  "clean-db",
		Usage: "if dagger encountered an error loading graphs into the database, this will drop orphan graph parts",
		Flags: dbFlags(),
		Action: func(c *cli.Context) error {
			mongodbURI := c.String("dbUri")
			dbName := c.String("dbName")

			env := sink.GetEnvironment()

			if err := configure(env, 2, true, mongodbURI, "", dbName); err != nil {
				return errors.WithStack(err)
			}

			graphs := &model.DependencyGraphs{}

			if err := graphs.FindIncomplete(); err != nil {
				return errors.Wrap(err, "encountered problem finding incomplete graphs")
			}

			if graphs.IsNil() || graphs.Size() == 0 {
				grip.Info("found no incomplete errors")
				return nil
			}

			catcher := grip.NewSimpleCatcher()
			for _, g := range graphs.Slice() {
				catcher.Add(g.RemoveNodes())
				catcher.Add(g.RemoveEdges())
			}

			if catcher.HasErrors() {
				grip.Warningf("encountered error removing %d incomplete graphs", graphs.Size())
				return catcher.Resolve()
			}

			grip.Infof("removed nodes and edges from %d incomplete graphs", graphs.Size())
			return nil
		},
	}

}
