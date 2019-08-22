package main

import (
	"context"

	"github.com/evergreen-ci/timber"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	grip.Log(level.Info, "running basic sender benchmark...")
	if err := timber.RunBasicSenderBenchmark(ctx); err != nil {
		grip.Error(err)
	}

	grip.Log(level.Info, "running flush benchmark...")
	if err := timber.RunFlushBenchmark(ctx); err != nil {
		grip.Error(err)
	}
}
