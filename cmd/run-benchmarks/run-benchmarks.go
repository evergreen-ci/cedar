package main

import (
	"context"
	"os"

	"github.com/evergreen-ci/cedar/benchmarks"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	arg := ""
	if len(os.Args) > 1 {
		arg = os.Args[1]
	}

	switch arg {
	case "timber":
		runTimberBasicSenderBenchmarks(ctx)
	case "download":
		runLogIteratorBenchmarks(ctx)
	default:
		runTimberBasicSenderBenchmarks(ctx)
		runLogIteratorBenchmarks(ctx)
	}
}

func runTimberBasicSenderBenchmarks(ctx context.Context) {
	grip.Log(level.Info, "running timber's basic sender benchmarks...")
	if err := benchmarks.RunBasicSenderBenchmark(ctx); err != nil {
		grip.Error(err)
	}
}

func runLogIteratorBenchmarks(ctx context.Context) {
	grip.Log(level.Info, "running log iterator benchmarks...")
	if err := benchmarks.RunLogIteratorBenchmark(ctx); err != nil {
		grip.Error(err)
	}
}
