package benchmarks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/pail"
	"github.com/evergreen-ci/poplar"
	"github.com/evergreen-ci/timber"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/mgo.v2/bson"
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
		return errors.Wrap(err, "problem creating top level directory")
	}

	logSizes := []int{1e5, 1e7, 1e9}
	var combinedReports string
	for _, logSize := range logSizes {
		id, err := uploadLog(ctx, logSize)
		if err != nil {
			return errors.Wrap(err, "problem uploading log")
		}

		clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
		client, err := mongo.Connect(ctx, clientOptions)
		if err != nil {
			return errors.Wrap(err, "problem connecting to mongo")
		}
		log := &model.Log{}
		err = client.Database(dbName).Collection("buildlogs").FindOne(ctx, bson.M{"_id": id}).Decode(log)
		if err != nil {
			return errors.Wrap(err, "problem fetching log from db")
		}

		suite, err := getLogIteratorBenchmarkSuite(ctx, log.Artifact)
		if err != nil {
			return errors.Wrap(err, "problem creating benchmark suite")
		}

		suitePrefix := filepath.Join(prefix, fmt.Sprintf("%d", logSize))
		if err = os.Mkdir(suitePrefix, os.ModePerm); err != nil {
			return errors.Wrap(err, "problem creating subdirectory")
		}

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

func uploadLog(ctx context.Context, logSize int) (string, error) {
	opts := &timber.LoggerOptions{
		Project:    "poplar-benchmarking",
		TaskID:     newUUID(),
		RPCAddress: rpcAddress,
		Insecure:   true,
	}
	logger, err := timber.MakeLogger(ctx, "benchmark", opts)
	if err != nil {
		return "", errors.Wrap(err, "problem creating buildlogger sender")
	}

	numLines := logSize / lineLength
	if numLines == 0 {
		numLines = 1
	}
	for i := 0; i < numLines; i++ {
		logger.Send(message.ConvertToComposer(level.Debug, newRandCharSetString(lineLength)))
	}

	return opts.GetLogID(), errors.Wrap(logger.Close(), "problem closing buildlogger sender")
}

func getLogIteratorBenchmarkSuite(ctx context.Context, artifact model.LogArtifactInfo) (poplar.BenchmarkSuite, error) {
	opts := pail.S3Options{
		Name:   bucketName,
		Prefix: artifact.Prefix,
		Region: "us-east-1",
	}
	bucket, err := pail.NewS3Bucket(opts)
	if err != nil {
		return poplar.BenchmarkSuite{}, errors.Wrap(err, "problem connecting to s3")
	}

	tr := util.TimeRange{
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
		r.Begin()
		for it.Next(ctx) {
			// do nothing
			_ = it.Item()
			r.IncOps(1)
		}
		r.End(time.Since(startAt))

		r.SetState(1)
		startAt = time.Now()
		r.Begin()
		err := it.Close()
		r.End(time.Since(startAt))
		r.IncOps(1)

		return errors.Wrap(err, "problem closing iterator")
	}
}
