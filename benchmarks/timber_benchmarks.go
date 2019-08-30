package benchmarks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/evergreen-ci/poplar"
	"github.com/evergreen-ci/timber"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

const (
	confFile   = "cedarconf.yaml"
	rpcAddress = "localhost:2289"
	dbName     = "poplar-benchmark"
	bucketName = "pail-s3-test"
	cedarBin   = "./build/cedar"
	lineLength = 200
)

// RunBasicSenderBenchmark runs a poplar benchmark suite for timber's basic
// sender implementation.
func RunBasicSenderBenchmark(ctx context.Context) error {
	srvCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := setupBenchmark(srvCtx); err != nil {
		return err
	}
	defer func() {
		if err := teardownBenchmark(srvCtx); err != nil {
			grip.Error(err)
		}
	}()

	prefix := filepath.Join(
		"build",
		fmt.Sprintf("basic_sender_benchmark_report_%d", time.Now().Unix()),
	)
	if err := os.Mkdir(prefix, os.ModePerm); err != nil {
		return errors.Wrap(err, "failed to create top level directory")
	}

	logSizes := []int{1e5, 1e7, 1e8}
	var combinedReports string
	for _, logSize := range logSizes {
		suitePrefix := filepath.Join(prefix, fmt.Sprintf("%d", logSize))
		if err := os.Mkdir(suitePrefix, os.ModePerm); err != nil {
			return errors.Wrap(err, "failed to create subdirectory")
		}

		suite := getBasicSenderSuite(logSize)
		results, err := suite.Run(ctx, suitePrefix)
		if err != nil {
			combinedReports += fmt.Sprintf("Log Size: %d\n===============\nError:\n%s\n", logSize, err)
			continue
		}

		combinedReports += fmt.Sprintf("Log Size: %d\n===============\n%s\n", logSize, results.Report())
	}

	f, err := os.Create(filepath.Join(prefix, "results.txt"))
	if err != nil {
		return errors.Wrap(err, "problem creating new file")
	}
	defer f.Close()

	_, err = f.WriteString(combinedReports)
	if err != nil {
		return errors.Wrap(err, "problem writing to file")
	}

	return nil
}

func getBasicSenderSuite(logSize int) poplar.BenchmarkSuite {
	return poplar.BenchmarkSuite{
		{
			CaseName:         "100KBBuffer",
			Bench:            getBasicSenderBenchmark(logSize, 1e5),
			MinRuntime:       time.Millisecond,
			MaxRuntime:       10 * time.Minute,
			Timeout:          20 * time.Minute,
			IterationTimeout: 10 * time.Minute,
			Count:            1,
			MinIterations:    1,
			MaxIterations:    2,
			Recorder:         poplar.RecorderPerf,
		},
		{

			CaseName:         "1MBBuffer",
			Bench:            getBasicSenderBenchmark(logSize, 1e6),
			MinRuntime:       time.Millisecond,
			MaxRuntime:       10 * time.Minute,
			Timeout:          20 * time.Minute,
			IterationTimeout: 10 * time.Minute,
			Count:            1,
			MinIterations:    1,
			MaxIterations:    2,
			Recorder:         poplar.RecorderPerf,
		},
		{

			CaseName:         "10MBBuffer",
			Bench:            getBasicSenderBenchmark(logSize, 1e7),
			MinRuntime:       time.Millisecond,
			MaxRuntime:       10 * time.Minute,
			Timeout:          20 * time.Minute,
			IterationTimeout: 10 * time.Minute,
			Count:            1,
			MinIterations:    1,
			MaxIterations:    2,
			Recorder:         poplar.RecorderPerf,
		},
		{

			CaseName:         "100MBBuffer",
			Bench:            getBasicSenderBenchmark(logSize, 1e8),
			MinRuntime:       time.Millisecond,
			MaxRuntime:       10 * time.Minute,
			Timeout:          20 * time.Minute,
			IterationTimeout: 10 * time.Minute,
			Count:            1,
			MinIterations:    1,
			MaxIterations:    2,
			Recorder:         poplar.RecorderPerf,
		},
	}
}

func getBasicSenderBenchmark(logSize, maxBufferSize int) poplar.Benchmark {
	numLines := logSize / lineLength
	if numLines == 0 {
		numLines = 1
	}
	lines := make([]string, numLines)
	for i := 0; i < numLines; i++ {
		lines[i] = newRandCharSetString(lineLength)
	}

	return func(ctx context.Context, r poplar.Recorder, _ int) error {
		opts := &timber.LoggerOptions{
			Project:       "poplar-benchmarking",
			TaskID:        newUUID(),
			RPCAddress:    rpcAddress,
			Insecure:      true,
			MaxBufferSize: maxBufferSize,
		}
		logger, err := timber.MakeLogger(ctx, "benchmark", opts)
		if err != nil {
			return errors.Wrap(err, "problem creating buildlogger sender")
		}

		r.SetState(0)
		for _, line := range lines {
			if err = ctx.Err(); err != nil {
				return errors.Wrap(err, "context error while sending")
			}
			m := message.ConvertToComposer(level.Debug, line)
			startAt := time.Now()
			r.Begin()
			logger.Send(m)
			r.End(time.Since(startAt))
			r.IncOps(1)
		}

		r.SetState(1)
		startAt := time.Now()
		r.Begin()
		if err = logger.Close(); err != nil {
			return errors.Wrap(err, "problem closing buildlogger sender")
		}
		r.End(time.Since(startAt))
		r.IncOps(1)

		return nil
	}
}
