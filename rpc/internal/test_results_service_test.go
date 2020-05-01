package internal

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestCreateTestResultsRecord(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env, err := createTestResultsEnv()
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, teardownTestResultsEnv(ctx, env))
	}()

	port := getPort()
	require.NoError(t, startTestResultsService(ctx, env, port))
	client, err := getTestResultsGRPCClient(ctx, fmt.Sprintf("localhost:%d", port), []grpc.DialOption{grpc.WithInsecure()})
	require.NoError(t, err)

	t.Run("NoConfig", func(t *testing.T) {
		info := &TestResultsInfo{
			Project: "project0",
			TaskId:  "task_id0",
		}
		modelInfo := info.Export()

		_, err = client.CreateTestResultsRecord(ctx, info)
		assert.Error(t, err)
		results := &model.TestResults{ID: modelInfo.ID()}
		results.Setup(env)
		assert.Error(t, results.Find(ctx))
	})
	conf := model.NewCedarConfig(env)
	require.NoError(t, conf.Save())
	t.Run("ConfigWithoutBucketType", func(t *testing.T) {
		info := &TestResultsInfo{
			Project: "project1",
			TaskId:  "task_id1",
		}
		modelInfo := info.Export()

		resp, err := client.CreateTestResultsRecord(ctx, info)
		require.NoError(t, err)
		results := &model.TestResults{ID: modelInfo.ID()}
		results.Setup(env)
		require.NoError(t, results.Find(ctx))
		assert.Equal(t, modelInfo.ID(), resp.TestResultsRecordId)
		assert.Equal(t, modelInfo, results.Info)
		assert.Equal(t, model.PailLocal, results.Artifact.Type)
		assert.True(t, time.Since(results.CreatedAt) <= time.Second)
	})
	conf.Setup(env)
	require.NoError(t, conf.Find())
	conf.Bucket.TestResultsBucketType = model.PailS3
	require.NoError(t, conf.Save())
	t.Run("ConfigWithBucketType", func(t *testing.T) {
		info := &TestResultsInfo{
			Project: "project2",
			TaskId:  "task_id2",
		}
		modelInfo := info.Export()

		resp, err := client.CreateTestResultsRecord(ctx, info)
		require.NoError(t, err)
		results := &model.TestResults{ID: modelInfo.ID()}
		results.Setup(env)
		require.NoError(t, results.Find(ctx))
		assert.Equal(t, modelInfo.ID(), resp.TestResultsRecordId)
		assert.Equal(t, modelInfo, results.Info)
		assert.Equal(t, conf.Bucket.TestResultsBucketType, results.Artifact.Type)
		assert.True(t, time.Since(results.CreatedAt) <= time.Second)
	})
}

func createTestResultsEnv() (cedar.Environment, error) {
	testDB := "test-results-service-test"
	env, err := cedar.NewEnvironment(context.Background(), testDB, &cedar.Configuration{
		MongoDBURI:    "mongodb://localhost:27017",
		DatabaseName:  testDBName,
		SocketTimeout: time.Minute,
		NumWorkers:    2,
	})

	return env, err
}

func teardownTestResultsEnv(ctx context.Context, env cedar.Environment) error {
	return errors.WithStack(env.GetDB().Drop(ctx))
}

func startTestResultsService(ctx context.Context, env cedar.Environment, port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return errors.WithStack(err)
	}

	s := grpc.NewServer()
	AttachTestResultsService(env, s)

	go func() {
		_ = s.Serve(lis)
	}()
	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	return nil
}

func getTestResultsGRPCClient(ctx context.Context, address string, opts []grpc.DialOption) (CedarTestResultsClient, error) {
	conn, err := grpc.DialContext(ctx, address, opts...)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	return NewCedarTestResultsClient(conn), nil
}
