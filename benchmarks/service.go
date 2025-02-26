package benchmarks

import (
	"context"
	"os"
	"os/exec"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/pail"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	yaml "gopkg.in/yaml.v2"
)

func setupBenchmark(ctx context.Context) error {
	conf := model.NewCedarConfig(cedar.GetEnvironment())
	conf.Bucket = model.BucketConfig{
		AWSKey:          os.Getenv("AWS_KEY"),
		AWSSecret:       os.Getenv("AWS_SECRET"),
		BuildLogsBucket: bucketName,
	}
	yaml, err := yaml.Marshal(conf)
	if err != nil {
		return errors.Wrap(err, "marshalling config into YAML")
	}

	f, err := os.Create(confFile)
	if err != nil {
		return errors.Wrap(err, "creating new file")
	}
	_, err = f.Write(yaml)
	if err != nil {
		catcher := grip.NewBasicCatcher()
		catcher.Wrap(f.Close(), "closing config file")
		catcher.Wrap(err, "writing config to file")
		return catcher.Resolve()
	} else if err = f.Close(); err != nil {
		return errors.WithStack(err)
	}

	args := []string{
		"admin",
		"conf",
		"load",
		"--file",
		confFile,
		"--dbName",
		dbName,
	}
	cmd := exec.Command(cedarBin, args...)
	if err = cmd.Run(); err != nil {
		return errors.Wrap(err, "loading Cedar config")
	}

	args = []string{
		"service",
		"--dbName",
		dbName,
		"--rpcDisableTLS",
	}
	cmd = exec.CommandContext(ctx, cedarBin, args...)
	if err = cmd.Start(); err != nil {
		return errors.Wrap(err, "starting local Cedar service")
	}

	return nil
}

func teardownBenchmark(ctx context.Context) error {
	catcher := grip.NewBasicCatcher()

	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		catcher.Wrap(err, "connecting to DB")
	} else {
		catcher.Wrap(client.Database(dbName).Drop(ctx), "dropping DB")
	}

	opts := pail.S3Options{
		Name:   bucketName,
		Region: "us-east-1",
	}
	bucket, err := pail.NewS3Bucket(ctx, opts)
	if err != nil {
		catcher.Wrap(err, "connecting to S3")
	} else {
		catcher.Wrap(bucket.RemovePrefix(ctx, ""), "removing S3 files")
	}

	catcher.Add(os.RemoveAll(confFile))

	return catcher.Resolve()
}
