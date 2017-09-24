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
			groups(),
			dot(),
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
			et := []depgraph.EdgeType{
				depgraph.ImplicitLibraryToLibrary,
				depgraph.LibraryToLibrary,
				depgraph.LibraryToSymbol,
			}

			// make graph library only
			libgraph := graph.Filter(et, []depgraph.NodeType{depgraph.Library, depgraph.Symbol})

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
				ndb := gdb.MakeNode(&node)
				if err = ndb.Insert(); err != nil {
					catcher.Add(err)
					continue
				}
				grip.Debugf("adding node for '%s'", ndb.ID)
			}

			for _, edge := range graph.Edges {
				edb := gdb.MakeEdge(&edge)
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

func groups() cli.Command {
	return cli.Command{
		Name:  "groups",
		Usage: "return list of dependency cycles/groups",
		Flags: depsFlags(
			cli.StringFlag{
				Name:  "output",
				Value: "cycleReport.json",
				Usage: "specify the path to the filtered library graph",
			},
			cli.StringFlag{
				Name:  "prune",
				Usage: "drop edges containing this string",
			},
			cli.StringFlag{
				Name:  "prefix",
				Value: "build/cached/",
				Usage: "specify a prefix for objects to remove",
			}),
		Action: func(c *cli.Context) error {
			fn := c.String("path")
			grip.Infoln("starting to load graph from:", fn)
			graph, err := depgraph.New("cli", fn)
			if err != nil {
				return errors.Wrap(err, "problem loading graph")
			}

			graph.Prune(c.String("prune"))
			graph.Annotate()

			et := []depgraph.EdgeType{
				depgraph.ImplicitLibraryToLibrary,
				depgraph.LibraryToLibrary,
			}

			libgraph := graph.Filter(et, []depgraph.NodeType{depgraph.Library})
			grip.Infof("filtered library dependency graph with %d nodes and %d edges",
				len(libgraph.Nodes), len(libgraph.Edges))

			report := depgraph.NewCycleReport(graph.Mapping(c.String("prefix")))
			grip.Infof("found %d cycles in graph with %d nodes",
				len(report.Cycles), len(report.Graph))

			return errors.Wrap(writeJSON(c.String("output"), report),
				"problem cycle report")
		},
	}
}

func dot() cli.Command {
	return cli.Command{
		Name:  "dot",
		Usage: "return dot format of a graph",
		Flags: depsFlags(
			cli.StringFlag{
				Name:  "output",
				Value: "libs",
				Usage: "specify the path to the filtered library graph, dot/json extensions added",
			},
			cli.StringFlag{
				Name:  "prefix",
				Value: "build/cached/",
				Usage: "specify a prefix for objects to remove",
			},
			cli.StringFlag{
				Name:  "prune",
				Usage: "drop edges containing this string",
			},
			cli.BoolFlag{
				Name:  "full",
				Usage: "render the full graph, otherwise focus on library relationships",
			}),
		Action: func(c *cli.Context) error {
			fn := c.String("path")
			grip.Infoln("starting to load graph from:", fn)
			graph, err := depgraph.New("cli", fn)
			if err != nil {
				return errors.Wrap(err, "problem loading graph")
			}

			graph.Annotate()

			if !c.Bool("full") {
				et := []depgraph.EdgeType{
					depgraph.ImplicitLibraryToLibrary,
					depgraph.LibraryToLibrary,
				}

				graph = graph.Filter(et, []depgraph.NodeType{depgraph.Library})
				grip.Infof("filtered library dependency graph with %d nodes and %d edges",
					len(graph.Nodes), len(graph.Edges))
			}

			graph.Prune(c.String("prune"))

			report := graph.Mapping(c.String("prefix"))

			grip.Info("generating dot file")

			dot := report.Dot()
			cycles := depgraph.NewCycleReport(report)

			grip.Infof("found %d cycles in graph with %d nodes",
				len(cycles.Cycles), len(cycles.Graph))

			grip.Info("writing dot file to disk")
			if err = writeString(c.String("output")+".dot", dot); err != nil {
				return errors.Wrap(err, "problem writing dot file")
			}

			if err = writeJSON(c.String("output")+".json", report); err != nil {
				return errors.Wrap(err, "problem writing json file")
			}

			if err = writeJSON(c.String("output")+"-cycles.json", cycles); err != nil {
				return errors.Wrap(err, "problem writing json file")
			}

			return nil

		},
	}
}
