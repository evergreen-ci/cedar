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
		return errors.Wrap(err, "problem marshalling conf into yaml")
	}

	f, err := os.Create(confFile)
	if err != nil {
		return errors.Wrap(err, "problem creating new file")
	}
	defer f.Close()
	_, err = f.Write(yaml)
	if err != nil {
		catcher := grip.NewBasicCatcher()
		catcher.Add(errors.WithStack(f.Close()))
		catcher.Add(errors.Wrap(err, "problem writing conf to file"))
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
		return errors.Wrap(err, "problem loading cedar config")
	}

	args = []string{
		"service",
		"--dbName",
		dbName,
		"--rpcDisableTLS",
	}
	cmd = exec.CommandContext(ctx, cedarBin, args...)
	if err = cmd.Start(); err != nil {
		return errors.Wrap(err, "problem starting local cedar service")
	}

	return nil
}

func teardownBenchmark(ctx context.Context) error {
	catcher := grip.NewBasicCatcher()

	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		catcher.Add(errors.Wrap(err, "problem connecting to mongo"))
	} else {
		catcher.Add(errors.Wrap(client.Database(dbName).Drop(ctx), "problem dropping db"))
	}

	opts := pail.S3Options{
		Name:   bucketName,
		Region: "us-east-1",
	}
	bucket, err := pail.NewS3Bucket(opts)
	if err != nil {
		catcher.Add(errors.Wrap(err, "problem connecting to s3"))
	} else {
		catcher.Add(errors.Wrap(bucket.RemovePrefix(ctx, ""), "problem removing s3 files"))
	}

	catcher.Add(os.RemoveAll(confFile))

	return catcher.Resolve()
}
