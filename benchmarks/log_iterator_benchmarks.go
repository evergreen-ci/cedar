package benchmarks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/pail"
	"github.com/evergreen-ci/poplar"
	"github.com/evergreen-ci/timber/buildlogger"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// RunLogIteratorBenchmark runs a poplar benchmark suite for the various
// implementation of cedar's LogIterator interface.
func RunLogIteratorBenchmark(ctx context.Context) error {
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
		fmt.Sprintf("log_iterator_benchmark_report_%d", time.Now().Unix()),
	)
	if err := os.Mkdir(prefix, os.ModePerm); err != nil {
		return errors.Wrap(err, "creating top level directory")
	}

	logSizes := []int{1e5}
	var combinedReports string
	for _, logSize := range logSizes {
		id, err := uploadLog(ctx, logSize)
		if err != nil {
			return errors.Wrap(err, "uploading log")
		}

		clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
		client, err := mongo.Connect(ctx, clientOptions)
		if err != nil {
			return errors.Wrap(err, "connecting to DB")
		}
		log := &model.Log{}
		err = client.Database(dbName).Collection("buildlogs").FindOne(ctx, bson.M{"_id": id}).Decode(log)
		if err != nil {
			return errors.Wrap(err, "fetching log from DB")
		}

		suite, err := getLogIteratorBenchmarkSuite(ctx, log.Artifact)
		if err != nil {
			return errors.Wrap(err, "creating benchmark suite")
		}

		suitePrefix := filepath.Join(prefix, fmt.Sprintf("%d", logSize))
		if err = os.Mkdir(suitePrefix, os.ModePerm); err != nil {
			return errors.Wrapf(err, "creating subdirectory '%s'", suitePrefix)
		}

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

func uploadLog(ctx context.Context, logSize int) (string, error) {
	opts := &buildlogger.LoggerOptions{
		Project:     "poplar-benchmarking",
		TaskID:      newUUID(),
		BaseAddress: rpcBaseAddr,
		RPCPort:     strconv.Itoa(rpcPort),
		Insecure:    true,
	}
	logger, err := buildlogger.MakeLogger("benchmark", opts)
	if err != nil {
		return "", errors.Wrap(err, "creating buildlogger sender")
	}

	numLines := logSize / lineLength
	if numLines == 0 {
		numLines = 1
	}
	for i := 0; i < numLines; i++ {
		logger.Send(message.ConvertToComposer(level.Debug, newRandCharSetString(lineLength)))
	}

	return opts.GetLogID(), errors.Wrap(logger.Close(), "closing buildlogger sender")
}

func getLogIteratorBenchmarkSuite(ctx context.Context, artifact model.LogArtifactInfo) (poplar.BenchmarkSuite, error) {
	opts := pail.S3Options{
		Name:   bucketName,
		Prefix: artifact.Prefix,
		Region: "us-east-1",
	}
	bucket, err := pail.NewS3Bucket(opts)
	if err != nil {
		return poplar.BenchmarkSuite{}, errors.Wrap(err, "connecting to S3")
	}

	tr := model.TimeRange{
		StartAt: artifact.Chunks[0].Start,
		EndAt:   artifact.Chunks[len(artifact.Chunks)-1].End,
	}
	return poplar.BenchmarkSuite{
		{
			CaseName:         "SerializedIterator",
			Bench:            getLogIteratorBenchmark(model.NewSerializedLogIterator(bucket, artifact.Chunks, tr)),
			MinRuntime:       time.Millisecond,
			MaxRuntime:       10 * time.Minute,
			Timeout:          20 * time.Minute,
			IterationTimeout: 10 * time.Minute,
			MinIterations:    1,
			MaxIterations:    2,
			Recorder:         poplar.RecorderPerf,
		},
		{

			CaseName:         "ParallelizedIterator",
			Bench:            getLogIteratorBenchmark(model.NewParallelizedLogIterator(bucket, artifact.Chunks, tr)),
			MinRuntime:       time.Millisecond,
			MaxRuntime:       10 * time.Minute,
			Timeout:          20 * time.Minute,
			IterationTimeout: 10 * time.Minute,
			MinIterations:    1,
			MaxIterations:    2,
			Recorder:         poplar.RecorderPerf,
		},
		{

			CaseName:         "BatchedIteratorBatchSize2",
			Bench:            getLogIteratorBenchmark(model.NewBatchedLogIterator(bucket, artifact.Chunks, 2, tr)),
			MinRuntime:       time.Millisecond,
			MaxRuntime:       10 * time.Minute,
			Timeout:          20 * time.Minute,
			IterationTimeout: 10 * time.Minute,
			MinIterations:    1,
			MaxIterations:    2,
			Recorder:         poplar.RecorderPerf,
		},
	}, nil
}

func getLogIteratorBenchmark(it model.LogIterator) poplar.Benchmark {
	return func(ctx context.Context, r poplar.Recorder, _ int) error {
		r.SetState(0)
		startAt := time.Now()
		r.BeginIteration()
		for it.Next(ctx) {
			// do nothing
			_ = it.Item()
			r.IncOperations(1)
		}
		r.EndIteration(time.Since(startAt))

		r.SetState(1)
		startAt = time.Now()
		r.BeginIteration()
		err := it.Close()
		r.EndIteration(time.Since(startAt))
		r.IncOperations(1)

		return errors.Wrap(err, "closing iterator")
	}
}
