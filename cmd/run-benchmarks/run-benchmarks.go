package main

import (
	"context"

	"github.com/evergreen-ci/cedar/benchmarks"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	/*
		grip.Log(level.Info, "running timber's basic sender benchmarks...")
		if err := benchmarks.RunBasicSenderBenchmark(ctx); err != nil {
			grip.Error(err)
		}
	*/

	grip.Log(level.Info, "running log iterator benchmarks...")
	if err := benchmarks.RunLogIteratorBenchmark(ctx); err != nil {
		grip.Error(err)
	}

}
