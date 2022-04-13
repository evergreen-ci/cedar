package benchmarks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/evergreen-ci/poplar"
	"github.com/evergreen-ci/timber/buildlogger"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

const (
	confFile    = "cedarconf.yaml"
	rpcBaseAddr = "localhost"
	rpcPort     = 2289
	dbName      = "poplar-benchmark"
	bucketName  = "pail-s3-test"
	cedarBin    = "./build/cedar"
	lineLength  = 200
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
		return errors.Wrapf(err, "creating top level directory '%s'", prefix)
	}

	logSizes := []int{1e5, 1e7, 1e8}
	var combinedReports string
	for _, logSize := range logSizes {
		suitePrefix := filepath.Join(prefix, fmt.Sprintf("%d", logSize))
		if err := os.Mkdir(suitePrefix, os.ModePerm); err != nil {
			return errors.Wrapf(err, "creating subdirectory '%s'", suitePrefix)
		}

		suite := getBasicSenderSuite(logSize)
		results, err := suite.Run(ctx, suitePrefix)
		if err != nil {
			combinedReports += fmt.Sprintf("Log Size: %d\n===============\nError:\n%s\n", logSize, err)
			continue
		}

		combinedReports += fmt.Sprintf("Log Size: %d\n===============\n%s\n", logSize, results.Report())
	}

	resultsFile := filepath.Join(prefix, "results.txt")
	f, err := os.Create(resultsFile)
	if err != nil {
		return errors.Wrapf(err, "creating new file '%s'", resultsFile)
	}
	defer f.Close()

	_, err = f.WriteString(combinedReports)
	if err != nil {
		return errors.Wrapf(err, "writing to file '%s'", resultsFile)
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
		opts := &buildlogger.LoggerOptions{
			Project:       "poplar-benchmarking",
			TaskID:        newUUID(),
			RPCPort:       strconv.Itoa(rpcPort),
			BaseAddress:   rpcBaseAddr,
			Insecure:      true,
			MaxBufferSize: maxBufferSize,
		}
		logger, err := buildlogger.MakeLogger("benchmark", opts)
		if err != nil {
			return errors.Wrap(err, "creating buildlogger sender")
		}

		r.SetState(0)
		for _, line := range lines {
			if err = ctx.Err(); err != nil {
				return errors.Wrap(err, "sending buildlogger lines")
			}
			m := message.ConvertToComposer(level.Debug, line)
			startAt := time.Now()
			r.BeginIteration()
			logger.Send(m)
			r.EndIteration(time.Since(startAt))
			r.IncOperations(1)
		}

		r.SetState(1)
		startAt := time.Now()
		r.BeginIteration()
		if err = logger.Close(); err != nil {
			return errors.Wrap(err, "closing buildlogger sender")
		}
		r.EndIteration(time.Since(startAt))
		r.IncOperations(1)

		return nil
	}
}
