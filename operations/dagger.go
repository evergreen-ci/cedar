package operations

import (
	"context"
	"fmt"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/depgraph"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/util"
	"github.com/google/uuid"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

// Deps exposes (or will expose) client functionality for the
// mongodb server library, file, and symbol dependency graph.
func Dagger() cli.Command {
	return cli.Command{
		Name:  "dagger",
		Usage: "access mongodb library dependency information",
		Subcommands: []cli.Command{
			filterLibrary(),
			loadGraphToDB(),
			findPaths(),
			cleanDB(),
			groups(),
			process(),
		},
	}
}

func filterLibrary() cli.Command {
	return cli.Command{
		Name:  "filter-library",
		Usage: "take a full graph and return only the library components",
		Flags: mergeFlags(depsFlags(), addOutputPath()),
		Action: func(c *cli.Context) error {
			fn := c.String(pathFlagName)
			graph, err := depgraph.New("cli", fn)
			if err != nil {
				return errors.Wrap(err, "problem loading graph")
			}

			graph.Annotate() // add in implicit deps

			// make graph library only
			libgraph := graph.Filter(
				[]depgraph.EdgeType{
					depgraph.ImplicitLibraryToLibrary,
					depgraph.LibraryToLibrary,
					depgraph.LibraryToSymbol,
				},
				[]depgraph.NodeType{
					depgraph.Library,
				})

			libgraph.Prune("third_party")

			return errors.Wrap(util.WriteJSON(c.String(outputFlagName), libgraph),
				"problem writing filtered graph")
		},
	}
}

func loadGraphToDB() cli.Command {
	return cli.Command{
		Name:  "load",
		Usage: "load a graph from a file into a local database",
		Flags: mergeFlags(dbFlags(), depsFlags()),
		Action: func(c *cli.Context) error {
			mongodbURI := c.String(dbURIFlag)
			dbName := c.String(dbNameFlag)
			fn := c.String("path")

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sc := newServiceConf(2, true, mongodbURI, "", dbName)
			sc.interactive = true

			if err := sc.setup(ctx); err != nil {
				return errors.WithStack(err)
			}

			env := cedar.GetEnvironment()
			graph, err := depgraph.New("cli", fn)
			if err != nil {
				return errors.Wrap(err, "problem loading graph")
			}

			if graph.BuildID == "" {
				graph.BuildID = uuid.New().String()
			}

			gdb := &model.GraphMetadata{
				BuildID: graph.BuildID,
			}
			gdb.Setup(env)
			if err = gdb.Save(); err != nil {
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
				if err = edb.Save(); err != nil {
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
			mongodbURI := c.String(dbURIFlag)
			dbName := c.String(dbNameFlag)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sc := newServiceConf(2, true, mongodbURI, "", dbName)
			sc.interactive = true

			if err := sc.setup(ctx); err != nil {
				return errors.WithStack(err)
			}

			env := cedar.GetEnvironment()
			graphs := &model.DependencyGraphs{}
			graphs.Setup(env)
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

func findPaths() cli.Command {
	return cli.Command{
		Name: "find-paths",
		Usage: fmt.Sprintln("find paths between two nodes on the graph",
			"uses gograph implementations of graph algorithms."),
		Flags: depsFlags(
			cli.StringFlag{
				Name:  "from",
				Usage: "specify the starting point in the graph",
			},
			cli.StringFlag{
				Name:  "to",
				Usage: "specify the traversal target",
			},
			cli.StringFlag{
				Name:  "output",
				Value: "pathsReport.json",
				Usage: "specify the path to the filtered library graph",
			},
			cli.BoolFlag{
				Name:  "print",
				Usage: "specify to write the output to standard out.",
			},
			cli.StringFlag{
				Name:  "prune",
				Usage: "drop edges containing this string",
			}),
		Action: func(c *cli.Context) error {
			fn := c.String("path")
			prune := c.String("prune")
			graph, err := depgraph.New("cli", fn)
			if err != nil {
				return errors.Wrap(err, "problem loading graph")
			}

			if prune != "" {
				graph.Prune(prune)
			}

			libgraph := graph.Filter( // [edges-to-keep], [nodes-to-keep]
				[]depgraph.EdgeType{
					depgraph.LibraryToLibrary,
					depgraph.ImplicitLibraryToLibrary,
				},
				[]depgraph.NodeType{depgraph.Library})

			libgraph.FlattenEdges()
			libgraph.Annotate()

			paths, err := libgraph.AllBetween(c.String("from"), c.String("to"))
			if err != nil {
				return errors.Wrap(err, "problem resolving paths")
			}

			if c.Bool("print") {
				return errors.WithStack(util.PrintJSON(paths))
			}

			return errors.Wrap(util.WriteJSON(c.String("output"), paths),
				"problem writhing path report")
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

			return errors.Wrap(util.WriteJSON(c.String("output"), report),
				"problem cycle report")
		},
	}
}

func process() cli.Command {
	return cli.Command{
		Name:  "process",
		Usage: "takes a dagger graph and filters, prunes, and renders several output formats",
		Flags: depsFlags(
			cli.StringFlag{
				Name:  "output, o",
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
			cli.BoolTFlag{
				Name:  "noCycle",
				Usage: "disables the cycle report",
			},
			cli.BoolTFlag{
				Name:  "noDot",
				Usage: "disables dot output",
			},
			cli.BoolTFlag{
				Name:  "full",
				Usage: "render the full graph, otherwise focus on library relationships",
			}),
		Action: func(c *cli.Context) error {
			fn := c.String("path")
			graph, err := depgraph.New("cli", fn)
			if err != nil {
				return errors.Wrap(err, "problem loading graph")
			}

			graph.Annotate()

			if c.BoolT("full") {
				graph = graph.Filter(
					[]depgraph.EdgeType{
						depgraph.ImplicitLibraryToLibrary,
						depgraph.LibraryToLibrary,
					},
					[]depgraph.NodeType{
						depgraph.Library,
					})
				grip.Infof("filtered library dependency to graph with %d nodes and %d edges",
					len(graph.Nodes), len(graph.Edges))
			}

			graph.Prune(c.String("prune"))
			report := graph.Mapping(c.String("prefix"))

			if err = util.WriteJSON(c.String("output")+".json", report); err != nil {
				return errors.Wrap(err, "problem writing json file")
			}

			if c.BoolT("noDot") {
				grip.Info("generating dot file")
				dot := report.Dot()
				grip.Info("writing dot file to disk")

				if err = util.WriteString(c.String("output")+".dot", dot); err != nil {
					return errors.Wrap(err, "problem writing dot file")
				}
			}

			if c.BoolT("noCycle") {
				cycles := depgraph.NewCycleReport(report)
				grip.Infof("found %d cycles in graph with %d nodes",
					len(cycles.Cycles), len(cycles.Graph))

				if err = util.WriteJSON(c.String("output")+"-cycles.json", cycles); err != nil {
					return errors.Wrap(err, "problem writing json file")
				}
			}

			return nil

		},
	}
}
