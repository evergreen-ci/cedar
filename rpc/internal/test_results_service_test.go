package internal

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	port = getPort()
	require.NoError(t, startTestResultsService(ctx, nil, port))
	invalidClient, err := getTestResultsGRPCClient(ctx, fmt.Sprintf("localhost:%d", port), []grpc.DialOption{grpc.WithInsecure()})
	require.NoError(t, err)

	t.Run("NoConfig", func(t *testing.T) {
		info := getTestResultsInfo()
		modelInfo, err := info.Export()
		require.NoError(t, err)

		resp, err := client.CreateTestResultsRecord(ctx, info)
		assert.Error(t, err)
		assert.Nil(t, resp)

		results := &model.TestResults{ID: modelInfo.ID()}
		results.Setup(env)
		assert.Error(t, results.Find(ctx))
	})
	conf := model.NewCedarConfig(env)
	require.NoError(t, conf.Save())
	t.Run("InvalidEnv", func(t *testing.T) {
		info := getTestResultsInfo()
		modelInfo, err := info.Export()
		require.NoError(t, err)

		resp, err := invalidClient.CreateTestResultsRecord(ctx, info)
		assert.Error(t, err)
		assert.Nil(t, resp)

		results := &model.TestResults{ID: modelInfo.ID()}
		results.Setup(env)
		assert.Error(t, results.Find(ctx))
	})
	t.Run("ConfigWithoutBucketType", func(t *testing.T) {
		info := getTestResultsInfo()
		modelInfo, err := info.Export()
		require.NoError(t, err)

		resp, err := client.CreateTestResultsRecord(ctx, info)
		assert.Error(t, err)
		assert.Nil(t, resp)

		results := &model.TestResults{ID: modelInfo.ID()}
		results.Setup(env)
		assert.Error(t, results.Find(ctx))
	})
	conf.Bucket.TestResultsBucketType = model.PailS3
	require.NoError(t, conf.Save())
	t.Run("ConfigWithBucketType", func(t *testing.T) {
		info := getTestResultsInfo()
		modelInfo, err := info.Export()
		require.NoError(t, err)

		resp, err := client.CreateTestResultsRecord(ctx, info)
		require.NoError(t, err)
		require.NotNil(t, resp)

		results := &model.TestResults{ID: modelInfo.ID()}
		results.Setup(env)
		require.NoError(t, results.Find(ctx))
		assert.Equal(t, modelInfo.ID(), resp.TestResultsRecordId)
		assert.Equal(t, modelInfo, results.Info)
		assert.Equal(t, conf.Bucket.TestResultsBucketType, results.Artifact.Type)
		assert.True(t, time.Since(results.CreatedAt) <= time.Second)
	})
}

func TestAddTestResults(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env, err := createTestResultsEnv()
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, teardownTestResultsEnv(ctx, env))
	}()
	tmpDir, err := ioutil.TempDir(".", "test-results-test")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
	}()

	conf := model.NewCedarConfig(env)
	require.NoError(t, conf.Save())

	info := getTestResultsInfo()
	exported, err := info.Export()
	require.NoError(t, err)
	record := model.CreateTestResults(exported, model.PailLocal)
	record.Setup(env)
	require.NoError(t, record.SaveNew(ctx))

	for _, test := range []struct {
		name        string
		results     *TestResults
		env         cedar.Environment
		invalidConf bool
		hasErr      bool
	}{
		{
			name: "InvalidEnv",
			results: &TestResults{
				TestResultsRecordId: record.ID,
				Results:             []*TestResult{getTestResult(), getTestResult(), getTestResult()},
			},
			hasErr: true,
		},
		{
			name: "DNE",
			results: &TestResults{
				TestResultsRecordId: "DNE",
				Results:             []*TestResult{getTestResult(), getTestResult(), getTestResult()},
			},
			env:    env,
			hasErr: true,
		},
		{
			name: "InvalidConfig",
			results: &TestResults{
				TestResultsRecordId: record.ID,
				Results:             []*TestResult{getTestResult(), getTestResult(), getTestResult()},
			},
			env:         env,
			invalidConf: true,
			hasErr:      true,
		},
		{
			name: "ValidData",
			results: &TestResults{
				TestResultsRecordId: record.ID,
				Results:             []*TestResult{getTestResult(), getTestResult(), getTestResult()},
			},
			env: env,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			port := getPort()
			require.NoError(t, startTestResultsService(ctx, test.env, port))
			client, err := getTestResultsGRPCClient(ctx, fmt.Sprintf("localhost:%d", port), []grpc.DialOption{grpc.WithInsecure()})
			require.NoError(t, err)

			if test.invalidConf {
				conf.Bucket.TestResultsBucket = ""
				conf.Bucket.PrestoBucket = ""
			} else {
				conf.Bucket.TestResultsBucket = tmpDir
				conf.Bucket.PrestoBucket = tmpDir
				conf.Bucket.PrestoTestResultsPrefix = "presto-test-results"
			}
			require.NoError(t, conf.Save())

			if test.hasErr {
				resp, err := client.AddTestResults(ctx, test.results)
				assert.Nil(t, resp)
				assert.Error(t, err)
			} else {
				resp, err := client.AddTestResults(ctx, test.results)
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, record.ID, resp.TestResultsRecordId)

				r := &model.TestResults{ID: resp.TestResultsRecordId}
				r.Setup(env)
				require.NoError(t, r.Find(ctx))
				assert.Equal(t, r.ID, r.Info.ID())
				results, err := r.Download(ctx)
				require.NoError(t, err)
				require.Len(t, results, len(test.results.Results))
				for i, result := range results {
					exportedResult := test.results.Results[i].Export()
					exportedResult.TaskID = record.Info.TaskID
					exportedResult.Execution = record.Info.Execution
					assert.Equal(t, exportedResult, result)
				}

				time.Sleep(time.Second)
				for _, res := range test.results.Results {
					htdInfo := model.HistoricalTestDataInfo{
						Project:     record.Info.Project,
						Variant:     record.Info.Variant,
						TaskName:    record.Info.DisplayTaskName,
						TestName:    res.DisplayTestName,
						RequestType: record.Info.RequestType,
						Date:        res.TestEndTime.AsTime(),
					}
					htd, err := model.CreateHistoricalTestData(htdInfo)
					require.NoError(t, err)
					htd.Setup(env)
					require.NoError(t, htd.Find(ctx))
				}
			}
		})
	}
}

func TestStreamTestResults(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env, err := createTestResultsEnv()
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, teardownTestResultsEnv(ctx, env))
	}()
	tmpDir, err := ioutil.TempDir(".", "test-results-test")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
	}()

	conf := model.NewCedarConfig(env)
	require.NoError(t, conf.Save())

	info := getTestResultsInfo()
	exported, err := info.Export()
	require.NoError(t, err)
	record1 := model.CreateTestResults(exported, model.PailLocal)
	record1.Setup(env)
	require.NoError(t, record1.SaveNew(ctx))
	info = getTestResultsInfo()
	exported, err = info.Export()
	require.NoError(t, err)
	record2 := model.CreateTestResults(exported, model.PailLocal)
	record2.Setup(env)
	require.NoError(t, record2.SaveNew(ctx))

	for _, test := range []struct {
		name          string
		results       []*TestResults
		env           cedar.Environment
		invalidConf   bool
		hasErr        bool
		expectedCount int
	}{
		{
			name: "InvalidEnv",
			results: []*TestResults{
				{
					TestResultsRecordId: record1.ID,
					Results:             []*TestResult{getTestResult(), getTestResult(), getTestResult()},
				},
			},
			hasErr: true,
		},
		{
			name: "DNE",
			results: []*TestResults{
				{
					TestResultsRecordId: "DNE",
					Results:             []*TestResult{getTestResult(), getTestResult(), getTestResult()},
				},
			},
			env:    env,
			hasErr: true,
		},
		{
			name: "InvalidConfig",
			results: []*TestResults{
				{
					TestResultsRecordId: record1.ID,
					Results:             []*TestResult{getTestResult(), getTestResult(), getTestResult()},
				},
			},
			env:         env,
			invalidConf: true,
			hasErr:      true,
		},
		{
			name: "DifferentIDs",
			results: []*TestResults{
				{
					TestResultsRecordId: record1.ID,
					Results:             []*TestResult{getTestResult(), getTestResult(), getTestResult()},
				},
				{
					TestResultsRecordId: record2.ID,
					Results:             []*TestResult{getTestResult(), getTestResult(), getTestResult()},
				},
			},
			env:    env,
			hasErr: true,
		},
		{
			name: "ValidData",
			results: []*TestResults{
				{
					TestResultsRecordId: record1.ID,
					Results:             []*TestResult{getTestResult(), getTestResult(), getTestResult()},
				},
				{
					TestResultsRecordId: record1.ID,
					Results:             []*TestResult{getTestResult(), getTestResult(), getTestResult()},
				},
			},
			env:           env,
			expectedCount: 9,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			port := getPort()
			require.NoError(t, startTestResultsService(ctx, test.env, port))
			client, err := getTestResultsGRPCClient(ctx, fmt.Sprintf("localhost:%d", port), []grpc.DialOption{grpc.WithInsecure()})
			require.NoError(t, err)

			if test.invalidConf {
				conf.Bucket.TestResultsBucket = ""
				conf.Bucket.PrestoBucket = ""
			} else {
				conf.Bucket.TestResultsBucket = tmpDir
				conf.Bucket.PrestoBucket = tmpDir
				conf.Bucket.PrestoTestResultsPrefix = "presto-test-results"
			}
			require.NoError(t, conf.Save())

			stream, err := client.StreamTestResults(ctx)
			require.NoError(t, err)

			catcher := grip.NewBasicCatcher()
			for i := 0; i < len(test.results); i++ {
				catcher.Add(stream.Send(test.results[i]))
			}
			resp, err := stream.CloseAndRecv()
			catcher.Add(err)

			if test.hasErr {
				assert.Nil(t, resp)
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, test.results[0].TestResultsRecordId, resp.TestResultsRecordId)

				r := &model.TestResults{ID: resp.TestResultsRecordId}
				r.Setup(env)
				require.NoError(t, r.Find(ctx))
				assert.Equal(t, r.ID, r.Info.ID())
				results, err := r.Download(ctx)
				require.NoError(t, err)
				combinedResults := []*TestResult{}
				for i := range test.results {
					combinedResults = append(combinedResults, test.results[i].Results...)
				}
				assert.Len(t, results, test.expectedCount)

				time.Sleep(time.Second)
				for _, results := range test.results {
					for _, res := range results.Results {
						htdInfo := model.HistoricalTestDataInfo{
							Project:     record1.Info.Project,
							Variant:     record1.Info.Variant,
							TaskName:    record1.Info.DisplayTaskName,
							TestName:    res.DisplayTestName,
							RequestType: record1.Info.RequestType,
							Date:        res.TestEndTime.AsTime(),
						}
						htd, err := model.CreateHistoricalTestData(htdInfo)
						require.NoError(t, err)
						htd.Setup(env)
						require.NoError(t, htd.Find(ctx))
					}
				}
			}
		})
	}
}

func TestCloseTestResultsRecord(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env, err := createTestResultsEnv()
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, teardownTestResultsEnv(ctx, env))
	}()
	tmpDir, err := ioutil.TempDir(".", "test-results-test")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
	}()

	info := getTestResultsInfo()
	exported, err := info.Export()
	require.NoError(t, err)
	record := model.CreateTestResults(exported, model.PailLocal)
	record.Setup(env)
	require.NoError(t, record.SaveNew(ctx))

	for _, test := range []struct {
		name   string
		env    cedar.Environment
		info   *TestResultsEndInfo
		hasErr bool
	}{
		{
			name:   "InvalidEnv",
			info:   &TestResultsEndInfo{TestResultsRecordId: record.ID},
			hasErr: true,
		},
		{
			name:   "DNE",
			env:    env,
			info:   &TestResultsEndInfo{TestResultsRecordId: "DNE"},
			hasErr: true,
		},
		{
			name: "ValidData",
			env:  env,
			info: &TestResultsEndInfo{TestResultsRecordId: record.ID},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			port := getPort()
			require.NoError(t, startTestResultsService(ctx, test.env, port))
			client, err := getTestResultsGRPCClient(ctx, fmt.Sprintf("localhost:%d", port), []grpc.DialOption{grpc.WithInsecure()})
			require.NoError(t, err)

			resp, err := client.CloseTestResultsRecord(ctx, test.info)
			if test.hasErr {
				assert.Nil(t, resp)
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)

				assert.Equal(t, test.info.TestResultsRecordId, resp.TestResultsRecordId)
				r := &model.TestResults{ID: resp.TestResultsRecordId}
				r.Setup(env)
				require.NoError(t, r.Find(ctx))
				assert.Equal(t, r.ID, r.Info.ID())
				assert.True(t, time.Since(r.CompletedAt) <= time.Second)
			}
		})
	}
}

func createTestResultsEnv() (cedar.Environment, error) {
	env, err := cedar.NewEnvironment(context.Background(), testDBName, &cedar.Configuration{
		MongoDBURI:    "mongodb://localhost:27017",
		DatabaseName:  testDBName,
		SocketTimeout: time.Minute,
		NumWorkers:    2,
		DisableCache:  true,
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

func getTestResultsInfo() *TestResultsInfo {
	return &TestResultsInfo{
		Project:         utility.RandomString(),
		Version:         utility.RandomString(),
		Variant:         utility.RandomString(),
		TaskName:        utility.RandomString(),
		DisplayTaskName: utility.RandomString(),
		TaskId:          utility.RandomString(),
		DisplayTaskId:   utility.RandomString(),
		Execution:       rand.Int31n(5),
		RequestType:     utility.RandomString(),
	}
}

func getTestResult() *TestResult {
	now := time.Now()
	return &TestResult{
		TestName:        utility.RandomString(),
		DisplayTestName: utility.RandomString(),
		GroupId:         utility.RandomString(),
		Trial:           rand.Int31n(10),
		Status:          "Pass",
		LineNum:         rand.Int31n(1000),
		TaskCreateTime:  &timestamppb.Timestamp{Seconds: now.Add(-time.Hour).Unix()},
		TestStartTime:   &timestamppb.Timestamp{Seconds: now.Add(-30 * time.Hour).Unix()},
		TestEndTime:     &timestamppb.Timestamp{Seconds: now.Unix()},
	}
}

func getInvalidTestResult() *TestResult {
	now := time.Now()
	return &TestResult{
		TestName:        utility.RandomString(),
		DisplayTestName: utility.RandomString(),
		GroupId:         utility.RandomString(),
		Trial:           rand.Int31n(10),
		Status:          "Pass",
		LineNum:         rand.Int31n(1000),
		TaskCreateTime:  &timestamppb.Timestamp{Seconds: -100000000000},
		TestStartTime:   &timestamppb.Timestamp{Seconds: now.Add(-30 * time.Hour).Unix()},
		TestEndTime:     &timestamppb.Timestamp{Seconds: now.Unix()},
	}
}
